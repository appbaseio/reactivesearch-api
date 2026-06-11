package storedquery

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(index, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{index}

	ctx := context.Background()

	// Check if the storedquery index already exists
	exists, err := util.GetClient7().IndexExists(index).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, index)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas))

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(index).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", index, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, index)
	return es, nil
}

// To get rules from ES index
func (es *elasticsearch) getStoredQueries(ctx context.Context) ([]ESStoredQueryDOC, error) {
	return es.getStoredQueriesEs7(ctx)
}

// To get rules from ES index
func (es *elasticsearch) validateQuery(ctx context.Context, q string) (*bool, error) {
	// only use query key to validate the query.
	// ES doesn't support to validate the `aggs` and other keys like `size`
	var queryAsMap map[string]interface{}
	err := json.Unmarshal([]byte(q), &queryAsMap)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, err
	}
	// if query key is nil then ignore validation
	if queryAsMap["query"] == nil {
		isValid := true
		return &isValid, nil
	}
	// validate query key
	var queryToValidate = map[string]interface{}{
		"query": queryAsMap["query"],
	}
	queryAsString, err := json.Marshal(queryToValidate)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, err
	}
	q = string(queryAsString)
	return es.validateQueryEs7(ctx, q)
}

// To update a stored query in ES index
func (es *elasticsearch) updateStoredQuery(ctx context.Context, queryID string, record ESStoredQueryDOC) error {
	_, err := util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexName).
		Id(queryID).
		BodyJson(record).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error updating stored query record:", err)
		return err
	}
	return nil
}

// To delete a stored query in ES index
func (es *elasticsearch) deleteStoredQuery(ctx context.Context, ruleID string) error {
	return es.deleteStoredQueryEs7(ctx, ruleID)
}

// To execute a query
func (es *elasticsearch) executeQuery(ctx context.Context, index, query string) ([]byte, error) {
	return es.executeQueryEs7(ctx, index, query)
}
