package applycache

import (
	"context"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/reactivesearch/util"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(rulesIndex, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{rulesIndex}

	ctx := context.Background()

	// Check if the rules index already exists
	exists, err := util.GetClient7().IndexExists(rulesIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, rulesIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, cacheMapping, util.HiddenIndexSettings(), replicas)

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(rulesIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", rulesIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, rulesIndex)
	return es, nil
}

// addStat will add the passed stat record to the ElasticSearch index
func (es *elasticsearch) addStat(ctx context.Context, statRecord CacheStatES) error {
	return es.addStat7(ctx, statRecord)
}

// updateStat will update the passed stat record in ElasticSearch index
// based on the passed ID.
func (es *elasticsearch) updateStatScript(ctx context.Context, scriptString string, params map[string]interface{}, dayID string) error {
	return es.updateStatScript7(ctx, scriptString, params, dayID)
}

// DeleteOlderRecordsByDate will delete older records based on the
// passed date
func (es *elasticsearch) deleteOlderRecordsByDate(ctx context.Context, maxTime int64) error {
	return es.deleteOlderRecordsByDate7(ctx, maxTime)
}

// isIdExists will check if the ID exists in the index
func (es *elasticsearch) isIdExists(ctx context.Context, id string) (bool, error) {
	return es.isIdExists7(ctx, id)
}
