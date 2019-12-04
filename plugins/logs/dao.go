package logs

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(indexName, config string) (*elasticsearch, error) {
	ctx := context.Background()

	var es = &elasticsearch{indexName}
	// Check if meta index already exists
	exists, err := util.GetClient7().IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named \"%s\" already exists, skipping ...\n", logTag, indexName)
		return es, nil
	}

	// set number_of_replicas to (nodes-1)
	nodes, err := util.GetTotalNodes()
	if err != nil {
		return nil, err
	}
	settings := fmt.Sprintf(config, nodes, nodes-1)

	// Meta index doesn't exist, create one
	_, err = util.GetClient7().CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named \"%s\"", indexName)
	}

	log.Printf("%s: successfully created index name \"%s\"", logTag, indexName)
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
		log.Printf("%s: error indexing log record: %v", logTag, err)
	}
}

func (es *elasticsearch) getRawLogs(ctx context.Context, from, size, filter string, indices ...string) ([]byte, error) {
	fmt.Println("calling get logs: ", from, size, filter, indices)
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
