package nodes

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/util"
)

type elasticsearch struct {
	indexName string
	mapping   string
}

func initPlugin(indexName, mapping string) (*elasticsearch, error) {
	ctx := context.Background()

	es := &elasticsearch{indexName, mapping}

	// Check if the meta index already exists
	exists, err := util.GetClient7().IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while checking if index already exists: %v", logTag, err)
	}
	if exists {
		log.Println(logTag, ": index named", indexName, "already exists, skipping...")
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas)

	// Create a new meta index
	_, err = util.GetClient7().CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while creating index named %s: %v", logTag, indexName, err)
	}

	log.Println(logTag, ": successfully created index named", indexName)
	return es, nil
}

// pingES will ping ElasticSearch with the machine ID and the current time
func (es *elasticsearch) pingES(ctx context.Context, machineID string) error {
	return es.pingES7(ctx, machineID)
}

// deleteOlderRecords will delete all records older than 7 days
func (es *elasticsearch) deleteOlderRecords(ctx context.Context) error {
	return es.deleteOlderRecords7(ctx)
}

// activeNodesInTenMins will get the number of active nodes in the last
// 10 mins.
func (es *elasticsearch) activeNodesInTenMins(ctx context.Context) (int64, error) {
	return es.activeNodesInTenMins7(ctx)
}
