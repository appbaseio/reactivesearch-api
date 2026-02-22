package applycache

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/escompat"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// addStat7 will add the passed stat in the ElasticSearch index.
func (es *elasticsearch) addStat7(ctx context.Context, statRecord CacheStatES) error {
	_, err := util.GetClient7().
		Index().
		Index(es.indexName).
		BodyJson(statRecord).
		Refresh("wait_for").
		Id(generateDayID()).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error indexing stat record:", err)
		return err
	}
	return nil
}

// updateStat7 will update the passed stat in the ElasticSearch index
func (es *elasticsearch) updateStatScript7(ctx context.Context, scriptString string, params map[string]interface{}, dayID string) error {
	// Create the script
	esScript := es7.NewScript(scriptString).Params(params)

	_, err := util.GetClient7().
		Update().
		Index(es.indexName).
		Id(dayID).
		Script(esScript).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error updating cache stat record:", err)
		return err
	}
	return nil
}

// DeleteOlderRecordsByDate7 will delete older records before the
// passed date.
func (es *elasticsearch) deleteOlderRecordsByDate7(ctx context.Context, maxTime int64) error {
	rangeQuery := escompat.NewRangeQuery("day").Lt(maxTime)

	_, err := util.GetClient7().DeleteByQuery().Index(es.indexName).Query(rangeQuery).Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error while deleting records older than 30 days, ", err)
		return err
	}

	return nil
}

// isIdExists7 will check if the passed ID exists in the index
func (es *elasticsearch) isIdExists7(ctx context.Context, id string) (bool, error) {
	response, err := util.GetClient7().Get().Index(es.indexName).Id(id).Do(ctx)
	if err != nil {
		return false, err
	}

	return response != nil, nil
}
