package applycache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/escompat"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// getCacheAnalytics will get the cache analytics based on the
// time values passed and accordingly return the stats for the
// cache.
//
// Use a range query to get stats for the time period passed.
func (es *elasticsearch) getCacheAnalytics(ctx context.Context, from string, to string) ([]byte, error) {
	// Parse the from and to values to int
	fromAsInt, fromParseErr := parseStringToUnix(from)
	if fromParseErr != nil {
		log.Warnln(logTag, ": error while parsing `from` time to int, ", fromParseErr)
		return nil, fromParseErr
	}

	toAsInt, toParseErr := parseStringToUnix(to)
	if toParseErr != nil {
		log.Warnln(logTag, ": error while parsing `to` time to int, ", toParseErr)
		return nil, toParseErr
	}

	rangeQuery := escompat.NewRangeQuery("day").Gte(fromAsInt).Lte(toAsInt)

	aggrMap := map[string]*es7.SumAggregation{
		"total_requests":            es7.NewSumAggregation().Field("cache_request_count"),
		"total_cache_hits":          es7.NewSumAggregation().Field("cache_hit"),
		"total_cache_misses":        es7.NewSumAggregation().Field("cache_miss"),
		"total_performance_savings": es7.NewSumAggregation().Field("performance_save"),
	}

	searchService := util.GetClient7().
		Search().
		Index(es.indexName).
		Query(rangeQuery)

	for aggrKey, aggrFunc := range aggrMap {
		searchService = searchService.Aggregation(aggrKey, aggrFunc)
	}

	result, err := searchService.Do(ctx)

	if err != nil {
		log.Warnln(logTag, ": error while getting cache analytics, ", err)
		return nil, err
	}

	// Do some manipulations to return a proper response
	cacheMap := make([]interface{}, 0)

	for _, hit := range result.Hits.Hits {
		// TODO: Is there need to handle an error?
		// elastic might already handle it?

		cacheEachHit := make(map[string]interface{})

		json.Unmarshal(hit.Source, &cacheEachHit)

		dayAsKey := cacheEachHit["day"]
		delete(cacheEachHit, "day")

		cacheEachHit["key"] = dayAsKey

		// Convert the ID to an int64
		timeAsInt, _ := strconv.ParseInt(hit.Id, 10, 64)

		keyAsTime := time.Unix(timeAsInt, 0)
		cacheEachHit["key_as_string"] = keyAsTime.Format("2006/01/02 15:04:05")

		cacheMap = append(cacheMap, cacheEachHit)
	}

	// Parse the total time as well
	cacheAnalyticsResponse := make(map[string]interface{})

	// Extract aggregation values
	for aggrKey, _ := range aggrMap {
		insertErr := insertAggrResult(result, &cacheAnalyticsResponse, aggrKey)
		if insertErr != nil {
			log.Warnln(logTag, ": ", insertErr.Error())
			return nil, insertErr
		}
	}

	// Parse the time string to make it contain the date in
	// dd/MM/YY format.
	cacheAnalyticsResponse["from_timestamp"] = parseTimeString(from)
	cacheAnalyticsResponse["to_timestamp"] = parseTimeString(to)

	cacheAnalyticsResponse["cache"] = cacheMap

	responseInBytes, marshalErr := json.Marshal(cacheAnalyticsResponse)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshalling response, ", marshalErr)
		return nil, marshalErr
	}

	return responseInBytes, nil
}

// insertAggrResult will insert the aggregation result
// from the ElasticSearch response into the analytics response
func insertAggrResult(esResponse *es7.SearchResult, cacheResponse *map[string]interface{}, aggregationKey string) error {
	aggregationValue, found := esResponse.Aggregations.Sum(aggregationKey)
	if !found {
		return fmt.Errorf("error occurred while finding value for `%s`", aggregationKey)
	}

	(*cacheResponse)[aggregationKey] = aggregationValue.Value
	return nil
}
