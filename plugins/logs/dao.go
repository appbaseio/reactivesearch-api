package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(alias, config string) (*elasticsearch, error) {
	ctx := context.Background()

	var es = &elasticsearch{alias}

	// Check if alias exists instead of index and create first index if not exists with `${alias}-000001`
	res, err := util.GetClient7().Aliases().Index("_all").Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	indices := res.IndicesByAlias(alias)
	exists := false
	if len(indices) > 0 {
		exists = true
	}

	if exists {
		log.Println(logTag, ": index named", alias, "already exists, skipping ...")
		return es, nil
	}

	// set number_of_replicas to (nodes-1)
	nodes, err := util.GetTotalNodes()
	if err != nil {
		return nil, err
	}
	settings := fmt.Sprintf(config, nodes, nodes-1)

	// Meta index doesn't exist, create one
	indexName := alias + `-000001`
	_, err = util.GetClient7().CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named \"%s\" %v", indexName, err)
	}

	log.Println(logTag, ": successfully created index name", indexName)

	// create alias for above created index
	addAliasActions := []es7.AliasAction{
		es7.NewAliasAddAction(alias).
			Index(indexName),
	}
	_, err = util.GetClient7().Alias().
		Action(addAliasActions...).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating alias \"%s\" %v", alias, err)
	}

	classify.SetIndexAlias(indexName, alias)
	classify.SetAliasIndex(alias, indexName)

	rolloverConditions := make(map[string]interface{})
	json.Unmarshal([]byte(rolloverConfig), &rolloverConditions)
	rolloverService, err := es7.NewIndicesRolloverService(util.GetClient7()).
		Alias(alias).
		Conditions(rolloverConditions).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating a rollover service \"%s\" %v", alias, err)
	}
	log.Println(logTag, ": rollover svc created ", rolloverService.Acknowledged)
	return es, nil
}

func (es *elasticsearch) indexRecord(ctx context.Context, rec record) {
	bulkIndex := es7.NewBulkIndexRequest().
		Index(es.indexName).
		Type("_doc").
		Doc(rec)

	_, err := util.GetClient7().Bulk().
		Add(bulkIndex).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error indexing log record :", err)
	}
}

func (es *elasticsearch) getRawLogs(ctx context.Context, from, size, filter string, indices ...string) ([]byte, error) {
	offset, err := strconv.Atoi(from)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "from"`, from)
	}
	s, err := strconv.Atoi(size)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "size"`, size)
	}
	switch util.GetVersion() {
	case 6:
		return es.getRawLogsES6(ctx, from, s, filter, offset, indices...)
	default:
		return es.getRawLogsES7(ctx, from, s, filter, offset, indices...)
	}
}

func (es *elasticsearch) rolloverIndex(alias string) {
	ctx := context.Background()
	log.Println(logTag, "=> checking if cron has exceeded")
	rolloverConditions := make(map[string]interface{})
	json.Unmarshal([]byte(rolloverConfig), &rolloverConditions)
	rolloverService, err := es7.NewIndicesRolloverService(util.GetClient7()).
		Alias(alias).
		Conditions(rolloverConditions).
		Do(ctx)
	if err != nil {
		log.Printf(logTag, "error while creating a rollover service %s %v", alias, err)
	}
	log.Println(logTag, ": rollover res", rolloverService.OldIndex, rolloverService.RolledOver)

	if rolloverService.RolledOver {
		util.GetClient7().DeleteIndex(rolloverService.OldIndex).Do(ctx)
		classify.RemoveFromIndexAliasCache(rolloverService.OldIndex)
		classify.SetIndexAlias(rolloverService.NewIndex, alias)
		classify.SetAliasIndex(alias, rolloverService.NewIndex)
	}
}
