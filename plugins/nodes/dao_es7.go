package nodes

import (
	"context"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/prometheus/common/log"
)

// pingES7 will ping ElasticSearch based on the passed machine ID
// with the current unix timestamp.
//
// This function will also determine whether the document should
// be created or updated based on the machineID being present
// or not being present in ES.
func (es *elasticsearch) pingES7(ctx context.Context, machineID string) error {
	// Get the current time
	currentTime := time.Now().Unix()
	pingDoc := ESNode{
		PingTime: &currentTime,
	}

	// Just sending an index request will suffice. If the ID will be present,
	// this request will update the doc or create one.
	_, err := util.GetClient7().
		Index().
		Index(es.indexName).
		BodyJson(pingDoc).
		Refresh("wait_for").
		Id(machineID).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error indexing ping time:", err)
		return err
	}
	return nil
}
