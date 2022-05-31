package nodes

import (
	"context"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
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

// deleteOlderRecords will ping ElasticSearch with a delete_by_query request
// where range will be used to delete records older than 7 days.
//
// We don't need to keep docs older than 7 days since we only need the node
// count of the last 10 mins and the last 7 days.
func (es *elasticsearch) deleteOlderRecords7(ctx context.Context) error {
	// Get the minimum time allowed.
	// We will get the current time - 7 days
	minTime := time.Now().AddDate(0, 0, -7).Unix()

	rangeQuery := es7.NewRangeQuery("ping_time").Lt(minTime)

	_, err := util.GetClient7().DeleteByQuery().Index(es.indexName).Query(rangeQuery).Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error while deleting records older than 7 days, ", err)
		return err
	}

	return nil
}

// activeNodesInTenMins will return the number of active nodes in the last
// 10 mins using the current time stamp and making a query against the
// nodes index.
//
// All docs that have `ping_time` set as greater than the last 10 mins will
// be fetched using this function and counted.
func (es *elasticsearch) activeNodesInTenMins7(ctx context.Context) (int64, error) {
	minTime := time.Now().Add(time.Minute * -10).Unix()

	rangeQuery := es7.NewRangeQuery("ping_time").Gte(minTime)

	resp, err := util.GetClient7().Count().Index(es.indexName).Query(rangeQuery).Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error while getting number of active nodes in the last 10 mins, ", err)
		return 0, err
	}

	return resp, nil
}

// activeNodesInSevenDays will return the number of active nodes in the last
// 7 days using the current time stamp and making a query against the nodes index.
//
// All docs that have `ping_time` set as greater than the last 7 days will be
// fetched using this function and counted.
func (es *elasticsearch) activeNodesInSevenDays(ctx context.Context) (int64, error) {
	minTime := time.Now().AddDate(0, 0, -7).Unix()

	rangeQuery := es7.NewRangeQuery("ping_time").Gte(minTime)

	nodeCount, err := util.GetClient7().Count().Index(es.indexName).Query(rangeQuery).Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error while getting the number of active nodes in the last 7 days, ", err)
		return 0, err
	}

	return nodeCount, err
}
