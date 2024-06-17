package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) updateRecordEs7(ctx context.Context, updateConfig UpdateConfig) *Error {
	// To update the record
	res, err := util.GetClient7().
		Update().
		Index(es.analyticsIndex).
		DocAsUpsert(true).
		RetryOnConflict(5).
		Id(updateConfig.DocID).
		Doc(updateConfig.Record).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating analytics record for id=", updateConfig.DocID, ":", err)
		code := http.StatusInternalServerError
		if res != nil {
			code = res.Status
		}
		return &Error{
			message: err,
			code:    code,
		}
	}
	// To update the calculated fields
	res2, err2 := util.GetClient7().
		Update().
		Index(es.analyticsIndex).
		RetryOnConflict(5).
		Id(updateConfig.DocID).
		Script(es7.NewScript(updateConfig.Script).Params(updateConfig.ScriptParams)).
		Do(ctx)
	if err2 != nil {
		log.Errorln(logTag, ": error updating analytics record for id=", updateConfig.DocID, ":", err2)
		code := http.StatusInternalServerError
		if res2 != nil {
			code = res2.Status
		}
		return &Error{
			message: err2,
			code:    code,
		}
	}
	return nil
}

func (es *elasticsearch) updateInsightStatusEs7(ctx context.Context, docID string, script string, scriptParams map[string]interface{}) error {
	_, err := util.GetClient7().
		Update().
		Index(es.insightsIndex).
		Upsert(make(map[string]interface{})).
		ScriptedUpsert(true).
		RetryOnConflict(5).
		Id(docID).
		Script(es7.NewScript(script).Params(scriptParams)).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating analytics insight status for id=", docID, ":", err)
		return err
	}
	return nil
}

func (es *elasticsearch) getInsightStatusEs7(ctx context.Context, docID string) (InsightStatusEsDoc, error) {
	var insightStatus InsightStatusEsDoc
	result, err := util.GetClient7().
		Get().
		Index(es.insightsIndex).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error while retrieving insight status for id=", docID, ":", err)
		return insightStatus, err
	}
	err2 := json.Unmarshal(result.Source, &insightStatus)
	if err2 != nil {
		log.Errorln(logTag, ": error while un-marshalling insight status for id=", docID, ":", err2)
		return insightStatus, err2
	}

	return insightStatus, nil
}

func (es *elasticsearch) updateUserSessionEs7(ctx context.Context, docID string, record UserSession, script string, scriptParams map[string]interface{}) error {
	_, err := util.GetClient7().
		Update().
		Index(es.userSessionIndex).
		ScriptedUpsert(true).
		Upsert(make(map[string]interface{})).
		RetryOnConflict(5).
		Script(es7.NewScript(script).Params(scriptParams)).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating user_session record for id=", docID, ":", err)
		return err
	}
	return nil
}

func (es *elasticsearch) storedQueriesUsageEs7(ctx context.Context, from, to string, size int, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("storedqueries.keyword").
		Size(size).
		OrderByCountDesc()

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("storedqueries_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch stored queries response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("storedqueries_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'storedqueries_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	storedQueries := make(map[string]interface{})
	if buckets == nil {
		storedQueries["storedqueries"] = []interface{}{}
	} else {
		storedQueries["storedqueries"] = buckets
	}
	return json.Marshal(storedQueries)
}

func (es *elasticsearch) queryRulesUsageEs7(ctx context.Context, from, to string, size int, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("queryrules.keyword").
		Size(size).
		OrderByCountDesc()

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("rules_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch query rules response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("rules_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'rules_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	storedQueries := make(map[string]interface{})
	if buckets == nil {
		storedQueries["rules"] = []interface{}{}
	} else {
		storedQueries["rules"] = buckets
	}
	return json.Marshal(storedQueries)
}

func (es *elasticsearch) popularSearchesRawEs7(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("search_query.keyword").
		Missing(emptyQueryLabel).
		Size(size).
		OrderByCountDesc()

	if clickAnalytics {
		applyClickAnalyticsOnTermseEs7(aggr)
	}
	// Apply search state
	applySearchStateOnTermsesEs7(aggr)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		TrackTotalHits(true).
		Aggregation("popular_searches_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular searches response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_searches_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'popular_searches_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		if clickAnalytics {
			newBucket = addClickAnalyticsEs7(bucket, bucket.DocCount, newBucket)
		}
		addSearchStateEs7(bucket, bucket.DocCount, newBucket)
		buckets = append(buckets, newBucket)
	}

	popularSearches := make(map[string]interface{})
	if buckets == nil {
		popularSearches["popular_searches"] = []interface{}{}
	} else {
		popularSearches["popular_searches"] = buckets
	}

	// Total number of searches
	popularSearches["total_searches"] = result.Hits.TotalHits.Value
	return json.Marshal(popularSearches)
}

func (es *elasticsearch) recentSearchesEs7(ctx context.Context, from, to string, size int, minChar *int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	if minChar != nil {
		minCharQuery := es7.NewRangeQuery("search_characters_length").Gte(minChar)
		query.Filter(minCharQuery)
	}

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	topHits := es7.NewTopHitsAggregation().Sort("timestamp", false).Size(1)

	aggr := es7.NewTermsAggregation().
		Field("search_query.keyword").
		Size(size).
		OrderByAggregation("timestamp_aggs.value", false).
		SubAggregation("timestamp_aggs", es7.NewMaxAggregation().Field("timestamp")).
		SubAggregation("timestamp_top_hits", topHits)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("recent_searches_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch recent searches response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("recent_searches_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'recent_searches_aggr'")
	}
	var buckets = make([]map[string]interface{}, 0)
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount

		timeStampToSet := ""

		// Extract the topHits
		topHits, topHitsOk := bucket.Aggregations.TopHits("timestamp_top_hits")
		if !topHitsOk {
			// Handle the error in case the top hits was not extracted.
			return nil, fmt.Errorf("error while getting the timestamp from the top hits aggregation")
		}

		if len(topHits.Hits.Hits) > 0 {
			// Parse the `timestamp` field from the source JSON byte.
			timestampAsString, timestampExtractErr := jsonparser.GetString(topHits.Hits.Hits[0].Source, "timestamp")

			// If there is an error, just ignore the error
			// Error should not happen in an ideal scenario
			if timestampExtractErr == nil {
				timeStampToSet = timestampAsString
			}
		}

		// Set the timestamp key to value
		newBucket["timestamp"] = timeStampToSet

		buckets = append(buckets, newBucket)
	}
	return json.Marshal(buckets)
}

func (es *elasticsearch) getTotalUniqueSearchesEs7(ctx context.Context, from, to string, minDocCount int, filters map[string]interface{}, indices ...string) (totalUniqueSearches float64, avgClickRate float64, err error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	if minDocCount == 0 {
		// Use cardinality aggregation to calculate the unique searches
		aggr := es7.NewCardinalityAggregation().
			Field("search_query.keyword")
		result, err2 := util.GetClient7().Search(es.analyticsIndex).
			Query(query).
			Aggregation("unique_searches_aggr", aggr).
			Do(ctx)
		if err2 != nil {
			err = fmt.Errorf("unable to fetch unique searches response from es: %v", err2)
			return
		}
		aggrResult, found := result.Aggregations.ValueCount("unique_searches_aggr")
		if found {
			if aggrResult.Value != nil {
				totalUniqueSearches = float64(*aggrResult.Value)
				return
			}
			return
		}
		err = fmt.Errorf("unable to fetch aggregation value from 'unique_searches_aggr'")
		return
	}
	// Use terms aggregation with min_doc_count to calculate the unique searches
	aggr := es7.NewTermsAggregation().
		Field("search_query.keyword").
		MinDocCount(minDocCount).
		Size(1000)

	applyClickAnalyticsOnTermseEs7(aggr)
	result, err2 := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("unique_searches_aggr", aggr).
		Do(ctx)
	if err2 != nil {
		err = fmt.Errorf("unable to fetch unique searches response from es: %v", err2)
		return
	}

	aggrResult, found := result.Aggregations.Terms("unique_searches_aggr")
	if !found {
		err = fmt.Errorf("unable to fetch aggregation value from 'unique_searches_aggr'")
		return
	}

	var totalClickRate float64
	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket = addClickAnalyticsEs7(bucket, bucket.DocCount, newBucket)
		clickRate, ok := newBucket["click_rate"].(float64)
		if !ok {
			err = fmt.Errorf("unable to parse click rate for 'unique_searches_aggr'")
			return
		}
		totalClickRate += clickRate
		buckets = append(buckets, newBucket)
	}
	totalUniqueSearches = float64(len(buckets))

	if totalUniqueSearches > 0 {
		avgClickRate = totalClickRate / totalUniqueSearches
	}
	return
}

func (es *elasticsearch) getTotalUniqueNoResultsSearchesEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (totalUniqueSearches float64, err error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	zeroHits := es7.NewTermQuery("total_hits", 0)

	query := es7.NewBoolQuery().Filter(duration, zeroHits)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	// Use cardinality aggregation to calculate the unique searches
	aggr := es7.NewCardinalityAggregation().
		Field("search_query.keyword").
		Missing(emptyQueryLabel)

	result, err2 := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("unique_searches_aggr", aggr).
		Do(ctx)
	if err2 != nil {
		err = fmt.Errorf("unable to fetch unique searches response from es: %v", err2)
		return
	}
	aggrResult, found := result.Aggregations.ValueCount("unique_searches_aggr")
	if found {
		if aggrResult.Value != nil {
			totalUniqueSearches = float64(*aggrResult.Value)
			return
		}
		return
	}
	err = fmt.Errorf("unable to fetch aggregation value from 'unique_searches_aggr'")
	return
}

func (es *elasticsearch) getTotalUniquePopularFiltersEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (totalUniqueFilters float64, err error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	// Use cardinality aggregation to calculate the unique filters
	aggr := es7.NewCardinalityAggregation().
		Field("search_filters.key.keyword")

	nestedAggr := es7.NewNestedAggregation().
		Path("search_filters").
		SubAggregation("unique_filters_aggr", aggr)

	result, err2 := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("unique_filters_aggr_nested", nestedAggr).
		Size(0).
		Do(ctx)
	if err2 != nil {
		err = fmt.Errorf("unable to fetch unique filters response from es: %v", err2)
		return
	}
	nestedAggrResult, found := result.Aggregations.Nested("unique_filters_aggr_nested")
	if found {
		aggrResult, found := nestedAggrResult.Aggregations.ValueCount("unique_filters_aggr")
		if found {
			if aggrResult.Value != nil {
				totalUniqueFilters = float64(*aggrResult.Value)
				return
			}
		}
		return
	}
	err = fmt.Errorf("unable to fetch aggregation value from 'unique_filters_aggr'")
	return
}

func (es *elasticsearch) getTotalFiltersSelectionsEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (totalUniqueFilters float64, err error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	// Use sum aggregation to calculate the total selections
	aggr := es7.NewValueCountAggregation().
		Field("search_filters.value.keyword")

	nestedAggr := es7.NewNestedAggregation().
		Path("search_filters").
		SubAggregation("total_selections_aggr", aggr)

	result, err2 := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("total_selections_aggr_nested", nestedAggr).
		Do(ctx)
	if err2 != nil {
		err = fmt.Errorf("unable to fetch unique filters response from es: %v", err2)
		return
	}
	nestedAggrResult, found := result.Aggregations.Nested("total_selections_aggr_nested")
	if found {
		aggrResult, found := nestedAggrResult.Aggregations.ValueCount("total_selections_aggr")
		if found {
			if aggrResult.Value != nil {
				totalUniqueFilters = float64(*aggrResult.Value)
				return
			}
		}
		return
	}
	err = fmt.Errorf("unable to fetch aggregation value from 'total_selections_aggr'")
	return
}

func (es *elasticsearch) getTotalSearchesFromLogsEs7(ctx context.Context, from, to string, minResponseTime *int, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	filterCategory := es7.NewTermQuery("category.keyword", "search")

	query := es7.NewBoolQuery().Filter(duration).Filter(filterCategory)

	if minResponseTime != nil {
		rangeFilter := es7.NewRangeQuery("response.took").Gt(*minResponseTime)
		query = query.Filter(rangeFilter)
	}

	util.GetIndexFilterQueryEs7(query, indices...)

	result, err2 := util.GetClient7().Count(es.logsIndex).
		Query(query).
		Do(ctx)
	if err2 != nil {
		return 0, fmt.Errorf("unable to fetch unique searches response from es: %v", err2)
	}
	return float64(result), nil
}

func (es *elasticsearch) noResultSearchesRawEs7(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	zeroHits := es7.NewTermQuery("total_hits", 0)

	query := es7.NewBoolQuery().Filter(duration, zeroHits)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("search_query.keyword").
		Missing(emptyQueryLabel).
		Size(size).
		OrderByCountDesc()

	// Apply search state
	applySearchStateOnTermsesEs7(aggr)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		TrackTotalHits(true).
		Size(0).
		Aggregation("no_results_searches_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch no results searches from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("no_results_searches_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'no_results_searches_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		addSearchStateEs7(bucket, bucket.DocCount, newBucket)
		buckets = append(buckets, newBucket)
	}

	noResultsSearches := make(map[string]interface{})
	if buckets == nil {
		noResultsSearches["no_results_searches"] = []interface{}{}
	} else {
		noResultsSearches["no_results_searches"] = buckets
	}
	noResultsSearches["total_searches"] = result.Hits.TotalHits.Value

	return json.Marshal(noResultsSearches)
}

func (es *elasticsearch) popularFiltersRawEs7(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	valueAggr := es7.NewTermsAggregation().
		Field("search_filters.value.keyword").
		Size(size).
		OrderByCountDesc()

	if clickAnalytics {
		applyClickAnalyticsOnFiltersEs7(valueAggr)
	}

	// apply search state
	applySearchStateOnFiltersEs7(valueAggr)

	aggr := es7.NewTermsAggregation().
		Field("search_filters.key.keyword").
		SubAggregation("values_aggr", valueAggr).
		OrderByCountDesc()

	nestedAggs := es7.NewNestedAggregation().Path("search_filters").SubAggregation("popular_filters_aggr", aggr)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("popular_filters_nested", nestedAggs).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular filters from es: %v", err)
	}

	nestedResult, found := result.Aggregations.Nested("popular_filters_nested")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'popular_filters_nested'")
	}

	aggrResult, found := nestedResult.Aggregations.Terms("popular_filters_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'popular_filters_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		valuesAggrResult, found := bucket.Terms("values_aggr")
		if !found {
			log.Println(logTag, ": unable to find 'values_aggr' in aggregation value")
			continue
		}
		for _, valueBucket := range valuesAggrResult.Buckets {
			newBucket := make(map[string]interface{})
			newBucket["key"] = bucket.Key
			newBucket["value"] = valueBucket.Key
			newBucket["count"] = valueBucket.DocCount
			if clickAnalytics {
				newBucket = addClickAnalyticsPopularFiltersEs7(valueBucket, valueBucket.DocCount, newBucket)
			}
			addSearchStateFiltersEs7(valueBucket, bucket.DocCount, newBucket)
			buckets = append(buckets, newBucket)
		}
	}

	sort.SliceStable(buckets, func(i, j int) bool {
		return buckets[i]["count"].(int64) > buckets[j]["count"].(int64)
	})

	popularFilters := make(map[string]interface{})
	if buckets == nil {
		popularFilters["popular_filters"] = []interface{}{}
	} else {
		popularFilters["popular_filters"] = buckets
	}

	return json.Marshal(popularFilters)
}

func (es *elasticsearch) popularResultsRawEs7(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	indexAggr := es7.NewTermsAggregation().
		Field("hits_in_response.index.keyword").Size(1).
		OrderByCountDesc()
	aggr := es7.NewTermsAggregation().
		Field("hits_in_response.id.keyword").
		Size(size).
		OrderByCountDesc().
		SubAggregation("index_aggr", indexAggr)

	if clickAnalytics {
		applyClickAnalyticsPopularResultsEs7(aggr)
	}
	// apply search state
	applySearchStateOnTermsesEs7(aggr)

	nestedAggs := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("popular_results_aggr", aggr)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("popular_results_nested", nestedAggs).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular results response from es: %v", err)
	}

	nestedResult, found := result.Aggregations.Nested("popular_results_nested")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'popular_results_nested'")
	}

	aggrResult, found := nestedResult.Aggregations.Terms("popular_results_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'popular_results_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		indexAggrResult, found := bucket.Aggregations.Terms("index_aggr")
		if !found {
			log.Println(logTag, ": unable to find 'index_aggr' in aggregation value")
			continue
		}
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		if indexAggrResult.Buckets != nil && len(indexAggrResult.Buckets) > 0 {
			newBucket["index"] = indexAggrResult.Buckets[0].Key
		}
		if clickAnalytics {
			newBucket = addClickAnalyticsPopularResultsEs7(bucket, bucket.DocCount, newBucket)
		}
		addSearchStateEs7(bucket, bucket.DocCount, newBucket)

		buckets = append(buckets, newBucket)
	}

	sort.SliceStable(buckets, func(i, j int) bool {
		return buckets[i]["count"].(int64) > buckets[j]["count"].(int64)
	})

	popularResults := make(map[string]interface{})
	if buckets == nil {
		popularResults["popular_results"] = []interface{}{}
	} else {
		popularResults["popular_results"] = buckets
	}

	return json.Marshal(popularResults)
}

func (es *elasticsearch) recentResultsEs7(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	indexAggr := es7.NewTermsAggregation().
		Field("hits_in_response.index.keyword").Size(1).
		OrderByCountDesc()
	aggr := es7.NewTermsAggregation().
		Field("hits_in_response.id.keyword").
		Size(size).
		OrderByAggregation("timestamp_reverse_aggs>timestamp_aggs.value", false).
		SubAggregation("index_aggr", indexAggr).
		SubAggregation("timestamp_reverse_aggs",
			es7.NewReverseNestedAggregation().
				SubAggregation("timestamp_aggs", es7.NewMaxAggregation().Field("timestamp")))

	nestedAggs := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("recent_results_aggr", aggr)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("recent_results_nested", nestedAggs).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch recent results response from es: %v", err)
	}

	nestedResult, found := result.Aggregations.Nested("recent_results_nested")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'recent_results_nested'")
	}

	aggrResult, found := nestedResult.Aggregations.Terms("recent_results_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'recent_results_aggr'")
	}

	var buckets = make([]map[string]interface{}, 0)
	for _, bucket := range aggrResult.Buckets {
		indexAggrResult, found := bucket.Aggregations.Terms("index_aggr")
		if !found {
			log.Println(logTag, ": unable to find 'index_aggr' in aggregation value")
			continue
		}
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		if indexAggrResult.Buckets != nil && len(indexAggrResult.Buckets) > 0 {
			newBucket["index"] = indexAggrResult.Buckets[0].Key
		}
		buckets = append(buckets, newBucket)
	}

	return json.Marshal(buckets)
}

func (es *elasticsearch) topResultsRawEs7(ctx context.Context, from, to, queryTerm string, size int, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	if queryTerm == "" {
		// filter top results for empty query
		query.Filter(es7.NewBoolQuery().MustNot(es7.NewExistsQuery("search_query.keyword")))
	} else {
		query.Filter(es7.NewTermQuery("search_query.keyword", queryTerm))
	}

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	indexAggr := es7.NewTermsAggregation().
		Field("hits_in_response.index.keyword").Size(1).
		OrderByCountDesc()
	aggr := es7.NewTermsAggregation().
		Field("hits_in_response.id.keyword").
		Size(size).
		OrderByCountDesc().
		SubAggregation("index_aggr", indexAggr)
	nestedAggr := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("popular_results_aggr", aggr)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("popular_results_aggr_nested", nestedAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular results response from es: %v", err)
	}

	nestedAggrResult, found := result.Aggregations.Nested("popular_results_aggr_nested")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'popular_results_aggr_nested'")
	}

	aggrResult, found := nestedAggrResult.Aggregations.Terms("popular_results_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'popular_results_aggr'")
	}

	var buckets = make([]map[string]interface{}, 0)
	for _, bucket := range aggrResult.Buckets {
		indexAggrResult, found := bucket.Aggregations.Terms("index_aggr")
		if !found {
			log.Println(logTag, ": unable to find 'index_aggr' in aggregation value")
			continue
		}
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		if indexAggrResult.Buckets != nil && len(indexAggrResult.Buckets) > 0 {
			newBucket["index"] = indexAggrResult.Buckets[0].Key
		}
		buckets = append(buckets, newBucket)
	}

	return buckets, nil
}

func (es *elasticsearch) topResultsClicksRawEs7(ctx context.Context, from, to, queryTerm string, size int, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	if queryTerm == "" {
		// filter top results for empty query
		query.Filter(es7.NewBoolQuery().MustNot(es7.NewExistsQuery("search_query.keyword")))
	} else {
		query.Filter(es7.NewTermQuery("search_query.keyword", queryTerm))
	}

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("hits_in_response.id.keyword").
		Size(size).
		OrderByCountDesc()

	filterAgg := es7.NewFilterAggregation().
		Filter(es7.NewTermQuery("hits_in_response.click", true)).
		SubAggregation("top_results_terms_aggr", aggr)

	nestedAggr := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("top_results_clicks_aggr", filterAgg)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("top_results_clicks_aggr_nested", nestedAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch top results clicks response from es: %v", err)
	}

	nestedAggrResult, found := result.Aggregations.Nested("top_results_clicks_aggr_nested")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'top_results_clicks_aggr_nested'")
	}

	filterAggrResult, found := nestedAggrResult.Aggregations.Filter("top_results_clicks_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'top_results_clicks_aggr'")
	}

	aggrResult, found := filterAggrResult.Aggregations.Terms("top_results_terms_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'top_results_terms_aggr'")
	}

	var buckets = make([]map[string]interface{}, 0)
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	return buckets, nil
}

func (es *elasticsearch) topSuggestionsClicksRawEs7(ctx context.Context, from, to, queryTerm string, size int, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	if queryTerm == "" {
		// filter top results for empty query
		query.Filter(es7.NewBoolQuery().MustNot(es7.NewExistsQuery("search_query.keyword")))
	} else {
		query.Filter(es7.NewTermQuery("search_query.keyword", queryTerm))
	}

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("suggestion_click_object_ids.keyword").
		Size(size).
		OrderByCountDesc()

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("top_suggestions_clicks_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch top suggestions clicks response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("top_suggestions_clicks_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'top_suggestions_clicks_aggr'")
	}

	var buckets = make([]map[string]interface{}, 0)
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	return buckets, nil
}

// for search state on the given terms aggregation
func applySearchStateOnTermsesEs7(aggr *es7.TermsAggregation) {
	searchStateAggr := es7.NewTopHitsAggregation().
		Size(1).
		FetchSourceContext(es7.NewFetchSourceContext(true).
			Include("search_state")).
		SortWithInfo(
			es7.SortInfo{
				Field:        "timestamp",
				UnmappedType: "date",
				Ascending:    false,
			})

	aggr.SubAggregation("topHits", searchStateAggr)
}

func applySearchStateOnFiltersEs7(aggr *es7.TermsAggregation) {
	searchStateAggr := es7.NewTopHitsAggregation().
		Size(1).
		FetchSourceContext(es7.NewFetchSourceContext(true).
			Include("search_state")).
		SortWithInfo(
			es7.SortInfo{
				Field:        "timestamp",
				UnmappedType: "date",
				Ascending:    false,
			})
	aggr.SubAggregation("topHits_reverse", es7.NewReverseNestedAggregation().SubAggregation("topHits", searchStateAggr))
}

// apply the custom events filters
func applyCustomEventsEs7(query *es7.BoolQuery, filters map[string]interface{}) {
	if len(filters) > 0 {
		var filterQueries []es7.Query
		for filter, value := range filters {
			field := AddEventPrefix(filter) + ".keyword"
			if IsPreDefinedTermFilter(filter) {
				// Don't add prefix for filters like `user_id` or `ip`
				field = filter + ".keyword"
			}
			query := es7.NewTermQuery(field, value)
			filterQueries = append(filterQueries, query)
		}
		query = query.Must(filterQueries...)
	}
}

// ApplyCustomEventsEs7 applies the custom events to ES7 query
func ApplyCustomEventsEs7(query *es7.BoolQuery, filters map[string]interface{}) {
	applyCustomEventsEs7(query, filters)
}

// apply the custom events filters for user sessions
func applyCustomEventsUserSessionEs7(query *es7.BoolQuery, filters map[string]interface{}) {
	if len(filters) > 0 {
		var filterQueries []es7.Query
		for filter, value := range filters {
			field := "custom_events." + AddEventPrefix(filter) + ".keyword"
			if IsPreDefinedTermFilter(filter) {
				// Don't add prefix for filters like `user_id` or `ip`
				field = filter + ".keyword"
				filterQueries = append(filterQueries, es7.NewTermQuery(field, value))
			} else {
				query := es7.NewNestedQuery("custom_events", es7.NewTermQuery(field, value))
				filterQueries = append(filterQueries, query)
			}

		}
		query = query.Must(filterQueries...)
	}
}

func addSearchStateEs7(bucket *es7.AggregationBucketKeyItem, count int64, newBucket map[string]interface{}) map[string]interface{} {
	topHits, _ := bucket.TopHits("topHits")
	type Source struct {
		SearchState interface{} `json:"search_state,omitempty"`
	}

	if topHits == nil || topHits.Hits == nil || topHits.Hits.Hits[0] == nil || topHits.Hits.Hits[0].Source == nil {
		newBucket["search_state"] = ""
		return newBucket
	}
	var source Source
	err := json.Unmarshal(topHits.Hits.Hits[0].Source, &source)
	if err != nil {
		log.Errorln(logTag, ": error un-marshaling search state:", err)
	}
	if source.SearchState != nil {
		newBucket["search_state"] = source.SearchState
	} else {
		newBucket["search_state"] = ""
	}
	return newBucket
}

func addSearchStateFiltersEs7(bucket *es7.AggregationBucketKeyItem, count int64, newBucket map[string]interface{}) map[string]interface{} {
	topHitsReverse, found := bucket.ReverseNested("topHits_reverse")
	if found {
		topHits, _ := topHitsReverse.TopHits("topHits")
		type Source struct {
			SearchState interface{} `json:"search_state,omitempty"`
		}

		if topHits == nil || topHits.Hits == nil || topHits.Hits.Hits[0] == nil || topHits.Hits.Hits[0].Source == nil {
			newBucket["search_state"] = ""
			return newBucket
		}
		var source Source
		err := json.Unmarshal(topHits.Hits.Hits[0].Source, &source)
		if err != nil {
			log.Errorln(logTag, ": error un-marshaling search state:", err)
		}
		if source.SearchState != nil {
			newBucket["search_state"] = source.SearchState
		} else {
			newBucket["search_state"] = ""
		}
	} else {
		newBucket["search_state"] = ""
	}

	return newBucket
}

// applyClickAnalyticsOnTermseEs7 is a mutator that applies aggregations
// for click analytics on the given terms aggregation.
func applyClickAnalyticsOnTermseEs7(aggr *es7.TermsAggregation) {
	clickAggr := es7.NewSumAggregation().
		Field("result_click_count")

	suggestionsClickAggr := es7.NewSumAggregation().
		Field("suggestion_click_count")

	clickPositionAggr := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("click_position_aggr", es7.NewAvgAggregation().
			Field("hits_in_response.click_position"))

	suggestionsClickPositionAggr := es7.NewAvgAggregation().
		Field("suggestion_click_position_ids")

	conversionAggr := es7.NewSumAggregation().
		Field("conversion_count")

	aggr.SubAggregation("click_aggr", clickAggr).
		SubAggregation("suggestions_click_aggr", suggestionsClickAggr).
		SubAggregation("click_position_nested_aggr", clickPositionAggr).
		SubAggregation("suggestions_click_position_aggr", suggestionsClickPositionAggr).
		SubAggregation("conversion_aggr", conversionAggr)
}

func applyClickAnalyticsOnFiltersEs7(aggr *es7.TermsAggregation) {
	clickAggr := es7.NewReverseNestedAggregation().
		SubAggregation("click_aggr",
			es7.NewSumAggregation().
				Field("result_click_count"))

	suggestionsClickAggr := es7.NewReverseNestedAggregation().
		SubAggregation("suggestions_click_aggr",
			es7.NewSumAggregation().
				Field("suggestion_click_count"))

	clickPositionAggr := es7.NewReverseNestedAggregation().
		SubAggregation("click_position_nested",
			es7.NewNestedAggregation().
				Path("hits_in_response").
				SubAggregation("click_position_aggr", es7.NewAvgAggregation().
					Field("hits_in_response.click_position")))

	suggestionsClickPositionAggr := es7.NewReverseNestedAggregation().
		SubAggregation("suggestions_click_position_aggr", es7.NewAvgAggregation().
			Field("suggestion_click_position_ids"))

	conversionAggr := es7.NewReverseNestedAggregation().
		SubAggregation("conversion_aggr", es7.NewSumAggregation().
			Field("conversion_count"))

	aggr.SubAggregation("click_aggr_reverse", clickAggr).
		SubAggregation("suggestions_click_aggr_reverse", suggestionsClickAggr).
		SubAggregation("click_position_aggr_reverse", clickPositionAggr).
		SubAggregation("suggestions_click_position_aggr_reverse", suggestionsClickPositionAggr).
		SubAggregation("conversion_aggr_reverse", conversionAggr)
}

func addClickAnalyticsPopularFiltersEs7(r *es7.AggregationBucketKeyItem, count int64, newBucket map[string]interface{}) map[string]interface{} {
	var resultClicks, suggestionClicks, avgResultClickPosition, avgSuggestionClickPosition, avgClickPosition float64
	clickAggrReverseResult, found := r.ReverseNested("click_aggr_reverse")
	if found {
		clickAggrResult, found := clickAggrReverseResult.Sum("click_aggr")
		if found {
			if clickAggrResult.Value != nil {
				resultClicks = *clickAggrResult.Value
			}
		}
	} else {
		log.Println(logTag, ": cannot find click aggregation value in aggregation value")
	}

	// suggestions click aggregation
	suggestionsClickAggrReverseResult, found := r.ReverseNested("suggestions_click_aggr_reverse")
	if found {
		suggestionsClickAggrResult, found := suggestionsClickAggrReverseResult.Sum("suggestions_click_aggr")
		if found {
			if suggestionsClickAggrResult.Value != nil {
				suggestionClicks = float64(*suggestionsClickAggrResult.Value)
			}
		}
	} else {
		log.Println(logTag, ": cannot find suggestions click aggregation value in aggregation value")
	}

	clickPositionAggrReverseResult, found := r.ReverseNested("click_position_aggr_reverse")
	if found {
		// click position aggregation
		clickPositionNestedAggrResult, found := clickPositionAggrReverseResult.Aggregations.Nested("click_position_nested")
		// click position aggregation
		if found {
			clickPositionAggrResult, found := clickPositionNestedAggrResult.Avg("click_position_aggr")
			if found {
				if clickPositionAggrResult.Value != nil {
					avgResultClickPosition = float64(*clickPositionAggrResult.Value)
				}
			} else {
				log.Println(logTag, ": cannot find click position aggregation value in aggregation value")
			}
		} else {
			log.Println(logTag, ": cannot find click position nested aggregation value in aggregation value")
		}
	}

	// suggestions click position aggregation
	suggestionsClickPositionAggrReverseResult, found := r.ReverseNested("suggestions_click_position_aggr_reverse")
	if found {
		suggestionsClickPositionAggrResult, found := suggestionsClickPositionAggrReverseResult.Avg("suggestions_click_position_aggr")
		if found {
			if suggestionsClickPositionAggrResult.Value != nil {
				avgSuggestionClickPosition = float64(*suggestionsClickPositionAggrResult.Value)
			} else {
				avgSuggestionClickPosition = float64(0) // TODO: default value 0?
			}
		} else {
			log.Println(logTag, ": cannot find suggestions click position aggregation value in aggregation value")
		}
	}

	// Calculate total clicks
	totalClicks := resultClicks + suggestionClicks
	// Calculate avg. click position
	if totalClicks != 0 {
		avgClickPosition = ((resultClicks * avgResultClickPosition) + (suggestionClicks * avgSuggestionClickPosition)) / totalClicks
	}
	// Conversion aggregation
	var conversionRate, clickRate, resultClickRate, suggestionClickRate, totalConversions float64
	conversionAggrReverseResult, found := r.ReverseNested("conversion_aggr_reverse")
	if found {
		conversionAggrResult, found := conversionAggrReverseResult.Sum("conversion_aggr")
		if found {
			if conversionAggrResult.Value != nil {
				totalConversions = float64(*conversionAggrResult.Value)
			} else {
				totalConversions = float64(0)
			}
		} else {
			log.Println(logTag, ": cannot find conversion aggregation value in aggregation value")
		}
	}

	if count != 0 {
		conversionRate = (float64(totalConversions) / float64(count)) * 100
		clickRate = float64(totalClicks) / float64(count) * 100 // TODO: check cast?
		resultClickRate = float64(resultClicks) / float64(count) * 100
		suggestionClickRate = float64(suggestionClicks) / float64(count) * 100
	}
	// Total Clicks
	newBucket["clicks"] = totalClicks
	// Result Clicks
	newBucket["result_clicks"] = resultClicks
	// Suggestion Clicks
	newBucket["suggestion_clicks"] = suggestionClicks
	// Avg. Click Position
	newBucket["click_position"] = avgClickPosition
	// Avg. Click Position Result
	newBucket["result_click_position"] = avgResultClickPosition
	// Avg. Click Position Suggestion
	newBucket["suggestion_click_position"] = avgSuggestionClickPosition
	// Click Rate
	newBucket["click_rate"] = clickRate
	// Result Click Rate
	newBucket["result_click_rate"] = resultClickRate
	// Suggestion Click Rate
	newBucket["suggestion_click_rate"] = suggestionClickRate
	// Conversion Rate
	newBucket["conversion_rate"] = conversionRate

	return newBucket
}

// TODO: TEST??
func addClickAnalyticsEs7(r *es7.AggregationBucketKeyItem, count int64, newBucket map[string]interface{}) map[string]interface{} {
	var resultClicks, suggestionClicks, avgResultClickPosition, avgSuggestionClickPosition, avgClickPosition float64

	// click aggregation
	clickAggrResult, found := r.Sum("click_aggr")
	if found {
		if clickAggrResult.Value != nil {
			resultClicks = *clickAggrResult.Value
		} else {
			resultClicks = 0
		}
	} else {
		log.Println(logTag, ": cannot find click aggregation value in aggregation value")
	}

	// suggestions click aggregation
	suggestionsClickAggrResult, found := r.Sum("suggestions_click_aggr")
	if found {
		if suggestionsClickAggrResult.Value != nil {
			suggestionClicks = float64(*suggestionsClickAggrResult.Value)
		} else {
			suggestionClicks = float64(0)
		}
	} else {
		log.Println(logTag, ": cannot find suggestions click aggregation value in aggregation value")
	}

	// click position aggregation
	clickPositionNestedAggrResult, found := r.Aggregations.Nested("click_position_nested_aggr")
	// click position aggregation
	if found {
		clickPositionAggrResult, found := clickPositionNestedAggrResult.Avg("click_position_aggr")
		if found {
			if clickPositionAggrResult.Value != nil {
				avgResultClickPosition = float64(*clickPositionAggrResult.Value)
			} else {
				avgResultClickPosition = float64(0) // TODO: default value 0?
			}
		} else {
			log.Println(logTag, ": cannot find click position aggregation value in aggregation value")
		}
	} else {
		log.Println(logTag, ": cannot find click position nested aggregation value in aggregation value")
	}

	// suggestions click position aggregation
	suggestionsClickPositionAggrResult, found := r.Avg("suggestions_click_position_aggr")
	if found {
		if suggestionsClickPositionAggrResult.Value != nil {
			avgSuggestionClickPosition = float64(*suggestionsClickPositionAggrResult.Value)
		} else {
			avgSuggestionClickPosition = float64(0) // TODO: default value 0?
		}
	} else {
		log.Println(logTag, ": cannot find suggestions click position aggregation value in aggregation value")
	}

	// Calculate total clicks
	totalClicks := resultClicks + suggestionClicks
	// Calculate avg. click position
	if totalClicks != 0 {
		avgClickPosition = ((resultClicks * avgResultClickPosition) + (suggestionClicks * avgSuggestionClickPosition)) / totalClicks
	}
	// Conversion aggregation
	var conversionRate, clickRate, resultClickRate, suggestionClickRate, totalConversions float64
	conversionAggrResult, found := r.Sum("conversion_aggr")
	if found {
		if conversionAggrResult.Value != nil {
			totalConversions = float64(*conversionAggrResult.Value)
		} else {
			totalConversions = float64(0)
		}
	} else {
		log.Println(logTag, ": cannot find conversion aggregation value in aggregation value")
	}

	if count != 0 {
		conversionRate = (float64(totalConversions) / float64(count)) * 100
		clickRate = float64(totalClicks) / float64(count) * 100 // TODO: check cast?
		resultClickRate = float64(resultClicks) / float64(count) * 100
		suggestionClickRate = float64(suggestionClicks) / float64(count) * 100
	}
	// Total Clicks
	newBucket["clicks"] = totalClicks
	// Result Clicks
	newBucket["result_clicks"] = resultClicks
	// Suggestion Clicks
	newBucket["suggestion_clicks"] = suggestionClicks
	// Avg. Click Position
	newBucket["click_position"] = avgClickPosition
	// Avg. Click Position Result
	newBucket["result_click_position"] = avgResultClickPosition
	// Avg. Click Position Suggestion
	newBucket["suggestion_click_position"] = avgSuggestionClickPosition
	// Click Rate
	newBucket["click_rate"] = clickRate
	// Result Click Rate
	newBucket["result_click_rate"] = resultClickRate
	// Suggestion Click Rate
	newBucket["suggestion_click_rate"] = suggestionClickRate
	// Conversion Rate
	newBucket["conversion_rate"] = conversionRate

	return newBucket
}

// applyClickAnalyticsOnTermseEs7 is a mutator that applies aggregations
// for click analytics on the given terms aggregation for popular results.
func applyClickAnalyticsPopularResultsEs7(aggr *es7.TermsAggregation) {
	clickAggr := es7.NewValueCountAggregation().
		Field("hits_in_response.click")

	clickPositionAggr := es7.NewAvgAggregation().
		Field("hits_in_response.click_position")

	conversionAggr := es7.NewSumAggregation().
		Field("hits_in_response.conversion")

	aggr.SubAggregation("click_aggr", clickAggr).
		SubAggregation("click_position_aggr", clickPositionAggr).
		SubAggregation("conversion_aggr", conversionAggr)
}

func addClickAnalyticsPopularResultsEs7(r *es7.AggregationBucketKeyItem, count int64, newBucket map[string]interface{}) map[string]interface{} {
	var resultClicks, avgResultClickPosition float64

	// click aggregation
	clickAggrResult, found := r.Sum("click_aggr")
	if found {
		if clickAggrResult.Value != nil {
			resultClicks = *clickAggrResult.Value
		} else {
			resultClicks = 0
		}
	} else {
		log.Println(logTag, ": cannot find click aggregation value in aggregation value")
	}

	// click position aggregation
	clickPositionAggrResult, found := r.Avg("click_position_aggr")
	if found {
		if clickPositionAggrResult.Value != nil {
			avgResultClickPosition = float64(*clickPositionAggrResult.Value)
		} else {
			avgResultClickPosition = float64(0) // TODO: default value 0?
		}
	} else {
		log.Println(logTag, ": cannot find click position aggregation value in aggregation value")
	}

	// Calculate total clicks
	totalClicks := resultClicks
	// Conversion aggregation
	var conversionRate, clickRate, totalConversions float64
	conversionAggrResult, found := r.Sum("conversion_aggr")
	if found && count != 0 {
		if conversionAggrResult.Value != nil {
			totalConversions = float64(*conversionAggrResult.Value)
		} else {
			totalConversions = float64(0)
		}
	} else {
		log.Println(logTag, ": cannot find conversion aggregation value in aggregation value")
	}
	if count != 0 {
		conversionRate = (float64(totalConversions) / float64(count)) * 100
		clickRate = float64(totalClicks) / float64(count) * 100
	}

	// Total Clicks
	newBucket["clicks"] = totalClicks
	// Avg. Click Position
	newBucket["click_position"] = avgResultClickPosition
	// Click Rate
	newBucket["click_rate"] = clickRate
	// Conversion Rate
	newBucket["conversion_rate"] = conversionRate

	return newBucket
}

func (es *elasticsearch) totalResultsCountEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	nestedAggs := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("popular_results_count_aggr", es7.NewValueCountAggregation().
			Field("hits_in_response.id.keyword"))

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Size(0).
		Query(query).
		Aggregation("popular_results_count_nested_aggr", nestedAggs).
		Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to fetch total results response from es: %v", err)
	}

	nestedAggrResult, found := result.Aggregations.Nested("popular_results_count_nested_aggr")
	if !found {
		return 0, fmt.Errorf("unable to fetch aggregation value from 'popular_results_count_nested_aggr'")
	}
	aggrResult, found := nestedAggrResult.Aggregations.ValueCount("popular_results_count_aggr")
	if !found {
		return 0, fmt.Errorf("unable to fetch aggregation value from 'popular_results_count_aggr'")
	}
	var resultsCount float64
	if aggrResult.Value != nil {
		resultsCount = *aggrResult.Value
	}
	return resultsCount, nil
}

func (es *elasticsearch) totalUniqueResultsCountEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewCardinalityAggregation().
		Field("hits_in_response.id.keyword")

	nestedAggr := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("popular_results_unique_count_aggr", aggr)

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Size(0).
		Query(query).
		Aggregation("popular_results_unique_count_nested_aggr", nestedAggr).
		Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to fetch total results response from es: %v", err)
	}

	nestedAggrResult, found := result.Aggregations.Nested("popular_results_unique_count_nested_aggr")
	if !found {
		return 0, fmt.Errorf("unable to fetch aggregation value from 'popular_results_unique_count_nested_aggr'")
	}

	aggrResult, found := nestedAggrResult.Aggregations.ValueCount("popular_results_unique_count_aggr")
	if !found {
		return 0, fmt.Errorf("unable to fetch aggregation value from 'popular_results_unique_count_aggr'")
	}
	var resultsCount float64
	if aggrResult.Value != nil {
		resultsCount = *aggrResult.Value
	}
	return resultsCount, nil
}

func (es *elasticsearch) getRequestDistributionEs7(ctx context.Context, queryParams QueryParams, interval string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(queryParams.From).
		To(queryParams.To)

	query := es7.NewBoolQuery().
		Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, err
	}
	intervalInSecs := int64(intervalDuration.Seconds())

	subAggr := es7.NewTermsAggregation().
		Field("response.code").
		OrderByCountDesc()
	aggr := es7.NewDateHistogramAggregation().
		FixedInterval(interval).
		Field("timestamp").
		SubAggregation("responses_with_code_aggr", subAggr)

	// add timezone to filter the buckets
	if queryParams.Timezone != "" {
		aggr = aggr.TimeZone(queryParams.Timezone)
	}

	// Assumes that logs plugin creates an index.
	result, err := util.GetClient7().Search(es.logsIndex).
		Query(query).
		Aggregation("request_distribution_aggr", aggr).
		Size(size).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	aggrResult, found := result.Aggregations.DateHistogram("request_distribution_aggr")
	if !found {
		return nil, fmt.Errorf(`unable to find aggregation value in "request_distribution_aggr"`)
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		subAggr, found := bucket.Terms("responses_with_code_aggr")
		if !found {
			log.Println(logTag, ": unable to find 'responses_with_code_aggr' in aggregation value")
			continue
		}
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["key_as_string"] = bucket.KeyAsString
		newBucket["count"] = bucket.DocCount
		newBucket["rpm"] = (bucket.DocCount * 60) / intervalInSecs

		var subBuckets []map[string]interface{}
		for _, bucket := range subAggr.Buckets {
			newSubBucket := make(map[string]interface{})
			newSubBucket["key"] = bucket.Key
			newSubBucket["count"] = bucket.DocCount
			subBuckets = append(subBuckets, newSubBucket)
		}
		if subBuckets == nil {
			subBuckets = []map[string]interface{}{}
		}

		newBucket["buckets"] = subBuckets
		buckets = append(buckets, newBucket)
	}

	requestDistribution := make(map[string]interface{})
	if buckets == nil {
		requestDistribution["request_distribution"] = []interface{}{}
	} else {
		requestDistribution["request_distribution"] = buckets
	}

	return json.Marshal(requestDistribution)
}

func (es *elasticsearch) getTotalRequestsEs7(ctx context.Context, from, to string, code int, indices ...string) (int64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().
		Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	if code != 0 {
		query.Filter(es7.NewTermQuery("response.code", code))
	}

	// Assumes that logs plugin creates an index.
	result, err := util.GetClient7().Search(es.logsIndex).
		Query(query).
		TrackTotalHits(true).
		Size(0).
		Do(ctx)
	if err != nil {
		return 0, err
	}
	return result.Hits.TotalHits.Value, nil
}

func (es *elasticsearch) latenciesEs7(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	aggr := es7.NewHistogramAggregation().
		Field("response.took").
		Interval(10)

	result, err := util.GetClient7().Search(es.logsIndex).
		Query(query).
		Aggregation("latency_aggr", aggr).
		Size(size).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch latency from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Histogram("latency_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'latency_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	latencies := make(map[string]interface{})
	if buckets == nil {
		latencies["latencies"] = []interface{}{}
	} else {
		latencies["latencies"] = buckets
	}

	return json.Marshal(latencies)
}

func (es *elasticsearch) getAvgSearchLatencyEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	aggr := es7.NewAvgAggregation().
		Field("response.took")

	result, err := util.GetClient7().Search(es.logsIndex).
		Query(query).
		Aggregation("avg_latency_aggr", aggr).
		Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to fetch avg. latency from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Avg("avg_latency_aggr")
	if !found {
		return 0, fmt.Errorf("unable to find aggregation value in 'avg_latency_aggr'")
	}

	if aggrResult.Value != nil {
		return util.WithPrecision(*aggrResult.Value, 2), nil
	}
	return 0, nil
}

func (es *elasticsearch) geoRequestsDistributionEs7(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("country.keyword").
		Size(size).
		OrderByCountDesc()

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("geo_dist_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch request distributions from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("geo_dist_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'req_dist_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		country, ok := bucket.Key.(string)
		if !ok {
			log.Println(logTag, ": invalid key type", bucket.Key, "received for country name")
			continue
		}
		if country != "" {
			newBucket := make(map[string]interface{})
			newBucket["key"] = country
			newBucket["count"] = bucket.DocCount
			buckets = append(buckets, newBucket)
		}
	}

	geoDist := make(map[string]interface{})
	if buckets == nil {
		geoDist["geo_distribution"] = []interface{}{}
	} else {
		geoDist["geo_distribution"] = buckets
	}

	return json.Marshal(geoDist)
}

func (es *elasticsearch) geoTotalCountriesEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewCardinalityAggregation().
		Field("country.keyword")

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("country_count_aggr", aggr).
		Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("unable to fetch request distributions from es: %v", err)
	}

	aggrResult, found := result.Aggregations.ValueCount("country_count_aggr")
	if !found {
		return 0, fmt.Errorf("unable to find aggregation value in 'country_count_aggr'")
	}

	if aggrResult.Value != nil {
		return *aggrResult.Value, nil
	}
	return 0, nil
}

func (es *elasticsearch) getFilterValuesEs7(ctx context.Context, label, prefix string, indices ...string) ([]byte, error) {
	query := es7.NewBoolQuery()
	util.GetIndexFilterQueryEs7(query, indices...)

	field := AddEventPrefix(label)
	if IsPreDefinedTermFilter(label) {
		// Don't add prefix for filters like `user_id` or `ip`
		field = label
	}

	filterAgg := es7.NewTermsAggregation().
		Field(field + ".keyword").
		Size(10).
		OrderByCountDesc()
	if prefix != "" {
		prefixQuery := es7.NewMatchPhrasePrefixQuery(field, prefix)
		query = query.Must(prefixQuery)
	}
	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("filter_label_aggregation", filterAgg).
		Do(ctx)

	aggrResult, found := result.Aggregations.Terms("filter_label_aggregation")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'filter_label_aggregation'")
	}
	values := make([]string, 0, len(aggrResult.Buckets))
	for _, bucket := range aggrResult.Buckets {
		if str, ok := bucket.Key.(string); ok {
			values = append(values, trimEventPrefix(str))
		}
	}
	if err != nil {
		return nil, err
	}

	filterValues := make(map[string]interface{})
	filterValues["filter_values"] = values

	return json.Marshal(filterValues)
}

func (es *elasticsearch) searchHistogramRawEs7(ctx context.Context, queryParams QueryParams, filters map[string]interface{}, indices ...string) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(queryParams.From).
		To(queryParams.To)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewDateHistogramAggregation().
		Interval("day").
		Field("timestamp")

	if queryParams.Timezone != "" {
		aggr = aggr.TimeZone(queryParams.Timezone)
	}

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("search_histogram_aggr", aggr).
		Size(0).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch date histogram from es: %v", err)
	}

	aggrResult, found := result.Aggregations.DateHistogram("search_histogram_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'search_histogram_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["key_as_string"] = bucket.KeyAsString
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	searchHistogram := make(map[string]interface{})
	if buckets == nil {
		searchHistogram["search_histogram"] = []interface{}{}
	} else {
		searchHistogram["search_histogram"] = buckets
	}

	return json.Marshal(searchHistogram)
}

func (es *elasticsearch) queryHistogramRawEs7(ctx context.Context, queryParams QueryParams, queryTerm string, filters map[string]interface{}, indices ...string) ([]map[string]interface{}, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(queryParams.From).
		To(queryParams.To)

	query := es7.NewBoolQuery().Filter(duration)
	if queryTerm == "" {
		// show histogram for empty query
		query.Filter(es7.NewBoolQuery().MustNot(es7.NewExistsQuery("search_query.keyword")))
	} else {
		query.Filter(es7.NewTermQuery("search_query.keyword", queryTerm))
	}

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewDateHistogramAggregation().
		FixedInterval("1d").
		Field("timestamp")

	if queryParams.Timezone != "" {
		aggr = aggr.TimeZone(queryParams.Timezone)
	}

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("query_histogram_aggr", aggr).
		Size(0).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch date histogram from es: %v", err)
	}

	aggrResult, found := result.Aggregations.DateHistogram("query_histogram_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'query_histogram_aggr'")
	}

	var buckets = make([]map[string]interface{}, 0)
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["key_as_string"] = bucket.KeyAsString
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	return buckets, nil
}

func (es *elasticsearch) totalSearchesEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewValueCountAggregation().Field("indices.keyword")

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("total_searches_aggr", aggr).
		Do(ctx)
	if err != nil {
		return 0, nil
	}

	aggrResult, found := result.Aggregations.ValueCount("total_searches_aggr")
	if !found {
		return 0, fmt.Errorf("unable to find aggregation value in 'total_searches_aggr'")
	}

	return *aggrResult.Value, nil
}

func (es *elasticsearch) totalUsersEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewCardinalityAggregation().Field("user_id.keyword")

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("total_users_aggr", aggr).
		Do(ctx)
	if err != nil {
		return 0, nil
	}

	aggrResult, found := result.Aggregations.ValueCount("total_users_aggr")
	if !found {
		return 0, fmt.Errorf("unable to find aggregation value in 'total_users_aggr'")
	}

	return *aggrResult.Value, nil
}

func (es *elasticsearch) totalUserSessionsEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	applyCustomEventsUserSessionEs7(query, filters)

	util.GetIndexFilterQueryEs7(query, indices...)

	aggr := es7.NewAvgAggregation().
		Field("duration")
	result, err := util.GetClient7().Search(es.userSessionIndex).
		Query(query).
		Size(0).
		TrackTotalHits(true).
		Aggregation("avg_duration_aggr", aggr).
		Do(ctx)
	if err != nil {
		return 0, 0, nil
	}

	aggrResult := result.Hits.TotalHits.Value

	avgDuration, found := result.Aggregations.Avg("avg_duration_aggr")
	if !found {
		return 0, 0, fmt.Errorf("unable to find average duration value in 'total_user_sessions_aggr'")
	}
	var avgDurationValue float64
	if avgDuration.Value != nil {
		avgDurationValue = *avgDuration.Value
	} else {
		avgDurationValue = 0
	}

	return float64(aggrResult), avgDurationValue, nil
}

func (es *elasticsearch) getActiveUserSessionsEs7(ctx context.Context) ([]ActiveUserSessionES, error) {
	activeUserSessions := []ActiveUserSessionES{}
	query := es7.NewRangeQuery("last_interaction_time").Gt(time.Now().Unix() - defaultUserSessionDuration*60)
	result, err := util.GetClient7().Search(es.userSessionIndex).
		Query(query).
		Size(10000).
		Do(ctx)
	if err != nil {
		return activeUserSessions, nil
	}
	for _, hit := range result.Hits.Hits {
		var userSession UserSession
		err := json.Unmarshal(hit.Source, &userSession)
		if err != nil {
			return activeUserSessions, err
		}
		activeUserSessions = append(activeUserSessions, ActiveUserSessionES{
			ID:          hit.Id,
			UserSession: userSession,
		})
	}
	return activeUserSessions, err
}

func (es *elasticsearch) avgResultClickPositionEs7(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	clickPositionAggr := es7.NewNestedAggregation().
		Path("hits_in_response").
		SubAggregation("click_position_aggr", es7.NewAvgAggregation().
			Field("hits_in_response.click_position"))

	query := es7.NewBoolQuery().Filter(duration)

	applyQueryFiltersEs7(query, queryFilters)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	result, err := util.GetClient7().
		Search(es.analyticsIndex).
		Query(query).
		Aggregation("avg-result-click-position-aggr", clickPositionAggr).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	aggrResult, found := result.Aggregations.Nested("avg-result-click-position-aggr")
	if found {
		clickAggResult, found := aggrResult.Aggregations.Avg("click_position_aggr")
		if found {
			if clickAggResult.Value != nil {
				return float64(*clickAggResult.Value), nil
			}
		}
		return 0, nil
	}
	return 0, fmt.Errorf("unable to fetch aggregation value from 'avg-result-click-position-aggr'")
}

func (es *elasticsearch) avgSuggestionsClickPositionEs7(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	clickPositionAggr := es7.NewAvgAggregation().
		Field("suggestion_click_position_ids")

	query := es7.NewBoolQuery().Filter(duration)

	applyQueryFiltersEs7(query, queryFilters)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	result, err := util.GetClient7().
		Search(es.analyticsIndex).
		Query(query).
		Aggregation("avg-suggestions-click-position-aggr", clickPositionAggr).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	aggrResult, found := result.Aggregations.Avg("avg-suggestions-click-position-aggr")
	if found {
		if aggrResult.Value != nil {
			return float64(*aggrResult.Value), nil
		}
		return 0, nil
	}
	return 0, fmt.Errorf("unable to fetch aggregation value from 'avg-suggestions-click-position-aggr'")
}

func (es *elasticsearch) totalBounceUsersEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	bounce := es7.NewTermQuery("bounce", true)

	query := es7.NewBoolQuery().Filter(duration).Filter(bounce)

	applyCustomEventsUserSessionEs7(query, filters)

	util.GetIndexFilterQueryEs7(query, indices...)

	result, err := util.GetClient7().Search(es.userSessionIndex).
		Query(query).
		TrackTotalHits(true).
		Size(0).
		Do(ctx)
	if err != nil {
		return 0, nil
	}

	return float64(result.Hits.TotalHits.Value), nil
}

func (es *elasticsearch) avgQueryLengthEs7(ctx context.Context, from, to string, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	queryLengthAggr := es7.NewAvgAggregation().
		Field("search_query_length")

	query := es7.NewBoolQuery().Filter(duration)
	util.GetIndexFilterQueryEs7(query, indices...)

	result, err := util.GetClient7().
		Search(es.analyticsIndex).
		Query(query).
		Size(0).
		Aggregation("query_length_aggr", queryLengthAggr).
		Do(ctx)
	if err != nil {
		return 0, err
	}
	aggrResult, found := result.Aggregations.Sum("query_length_aggr")
	if found {
		if aggrResult.Value == nil {
			return 0, nil
		}
		return float64(*aggrResult.Value), nil
	}
	return 0, fmt.Errorf("unable to fetch aggregation value from 'query_length_aggr'")
}

func (es *elasticsearch) noResultsSearchesEs7(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	zeroHits := es7.NewTermQuery("total_hits", 0)

	query := es7.NewBoolQuery().Filter(duration, zeroHits)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	aggr := es7.NewValueCountAggregation().Field("indices.keyword")

	result, err := util.GetClient7().Search(es.analyticsIndex).
		Query(query).
		Aggregation("no_results_searches_aggr", aggr).
		Do(ctx)
	if err != nil {
		return 0, nil
	}

	aggrResult, found := result.Aggregations.ValueCount("no_results_searches_aggr")
	if !found {
		return 0, fmt.Errorf("unable to find aggregation value in 'no_results_searches_aggr'")
	}

	return *aggrResult.Value, nil
}

func (es *elasticsearch) totalConversionsEs7(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	conversionAggr := es7.NewSumAggregation().
		Field("conversion_count")

	query := es7.NewBoolQuery().Filter(duration)

	applyQueryFiltersEs7(query, queryFilters)

	if indices != nil && len(indices) > 0 {
		var indexQueries []es7.Query
		for _, index := range indices {
			query := es7.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}
	applyCustomEventsEs7(query, filters)

	result, err := util.GetClient7().
		Search(es.analyticsIndex).
		Query(query).
		Aggregation("conversion_aggr", conversionAggr).
		Do(ctx)
	if err != nil {
		return 0, err
	}
	aggrResult, found := result.Aggregations.Sum("conversion_aggr")
	if found {
		if aggrResult.Value == nil {
			return 0, nil
		}
		return float64(*aggrResult.Value), nil
	}
	return 0, fmt.Errorf("unable to fetch aggregation value from 'conversion_aggr'")
}

func applyQueryFiltersEs7(query *es7.BoolQuery, queryFilters *[]QueryFilter) {
	if queryFilters != nil && len(*queryFilters) > 0 {
		for _, queryFilter := range *queryFilters {
			if queryFilter.Type == "exists" {
				if queryFilter.NestedPath != "" {
					query.Filter(es7.NewNestedQuery(queryFilter.NestedPath, es7.NewExistsQuery(queryFilter.Field)))
				} else {
					query.Filter(es7.NewExistsQuery(queryFilter.Field))
				}
			}
		}
	}
}

func (es *elasticsearch) totalClicksEs7(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	clickAggr := es7.NewSumAggregation().
		Field("result_click_count")

	query := es7.NewBoolQuery().Filter(duration)

	// apply query filters
	applyQueryFiltersEs7(query, queryFilters)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	result, err := util.GetClient7().
		Search(es.analyticsIndex).
		Query(query).
		Aggregation("total_clicks_aggr", clickAggr).
		Do(ctx)
	if err != nil {
		return 0, err
	}
	aggrResult, found := result.Aggregations.Sum("total_clicks_aggr")
	if found {
		if aggrResult.Value == nil {
			return 0, nil
		}
		return float64(*aggrResult.Value), nil
	}
	return 0, fmt.Errorf("unable to fetch aggregation value from 'total_clicks_aggr'")
}

func (es *elasticsearch) totalSuggestionsClicksEs7(ctx context.Context, from, to string, filters map[string]interface{}, queryFilters *[]QueryFilter, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	clickAggr := es7.NewSumAggregation().
		Field("suggestion_click_count")

	query := es7.NewBoolQuery().Filter(duration)

	applyQueryFiltersEs7(query, queryFilters)

	util.GetIndexFilterQueryEs7(query, indices...)

	applyCustomEventsEs7(query, filters)

	result, err := util.GetClient7().
		Search(es.analyticsIndex).
		Query(query).
		Aggregation("total_suggestions_clicks_aggr", clickAggr).
		Do(ctx)
	if err != nil {
		return 0, err
	}
	aggrResult, found := result.Aggregations.Sum("total_suggestions_clicks_aggr")
	if found {
		if aggrResult.Value == nil {
			return 0, nil
		}
		return float64(*aggrResult.Value), nil
	}
	return 0, fmt.Errorf("unable to fetch aggregation value from 'total_suggestions_clicks_aggr'")
}

func (es *elasticsearch) totalErrorsByStatusEs7(ctx context.Context, from, to string, minStatusCode *int, maxStatusCode *int, indices ...string) (float64, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	filterByStatus := es7.NewRangeQuery("response.code")
	if minStatusCode != nil {
		filterByStatus = filterByStatus.Gte(minStatusCode)
	}

	if maxStatusCode != nil {
		filterByStatus = filterByStatus.Lte(maxStatusCode)
	}

	query := es7.NewBoolQuery().Filter(duration).Filter(filterByStatus)

	util.GetIndexFilterQueryEs7(query, indices...)

	errorsAggr := es7.NewValueCountAggregation().Field("response.code")

	result, err := util.GetClient7().
		Search(es.logsIndex).
		Query(query).
		Aggregation("total_errors_aggr", errorsAggr).
		Do(ctx)
	if err != nil {
		return 0, err
	}
	aggrResult, found := result.Aggregations.ValueCount("total_errors_aggr")
	if found {
		if aggrResult.Value == nil {
			return 0, nil
		}
		return float64(*aggrResult.Value), nil
	}
	return 0, fmt.Errorf("unable to fetch aggregation value from 'total_errors_aggr'")
}

func (es *elasticsearch) getAdminUsersEs7(ctx context.Context) (*[]User, error) {
	query := es7.NewTermQuery("categories", "analytics")
	response, err := util.GetClient7().Search().
		Index(es.userIndex).
		Query(query).
		Size(1000).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	var users []User
	for _, hit := range response.Hits.Hits {
		var user User
		err := json.Unmarshal(hit.Source, &user)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return &users, nil
}

// savePreferences will save the passed preferences for
// analytics
func (es *elasticsearch) savePreferencesEs7(ctx context.Context, preferences AnalyticsPreferences) error {
	_, err := util.GetClient7().
		Update().
		Index(es.preferencesIndex).
		Upsert(preferences).
		DocAsUpsert(true).
		Doc(preferences).
		RetryOnConflict(5).
		Id(preferencesDocId).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating analytics preferences config :", err)
		return err
	}
	return nil
}

// getPreferencesEs7 will return the preferences for analytics
func (es *elasticsearch) getPreferencesEs7(ctx context.Context) (AnalyticsPreferences, error) {
	var record = AnalyticsPreferences{}
	response, err := util.GetClient7().Get().
		Index(es.preferencesIndex).
		Id(preferencesDocId).
		Do(ctx)
	if err != nil {
		log.Warnln(logTag, ": preferences not found", err)
		return defaultPreferences(), nil
	}

	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error retrieving analytics preferences", err)
		return record, err
	}
	return record, nil
}

// createRecentDocument will add a new document to the index
func (es recentDocumentsElasticsearch) createRecentDocument(ctx context.Context, record RecentDocument, docId string) error {
	_, err := util.GetClient7().
		Index().
		Index(es.index).
		BodyJson(record).
		Refresh("wait_for").
		Id(docId).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error indexing recent document record:", err)
		return err
	}
	return nil
}

// updateRecentDocumentWithUserId will inject the userId in a recent document
func (es recentDocumentsElasticsearch) updateRecentDocumentWithUserId(ctx context.Context, docId string, userId string) error {
	updatedAt := time.Now().Unix()

	_, updateErr := util.GetClient7().
		Update().
		Index(es.index).
		Id(docId).
		Script(es7.NewScriptInline(fmt.Sprintf("ctx._source.users['%s'] = %d", userId, updatedAt))).
		Do(ctx)

	if updateErr != nil {
		log.Errorln(logTag, ": error inserting userId: ", updateErr.Error())
		return updateErr
	}

	return nil
}

// doesDocumentExist will check if the document with the passed Id
// exists and accordingly return `true` or `false`.
func (es recentDocumentsElasticsearch) doesDocumentExist(ctx context.Context, docId string) (bool, error) {
	searchResponse, searchErr := util.GetClient7().
		Search().
		Index(es.index).
		Size(0).
		Query(es7.NewTermQuery("_id", docId)).
		Do(ctx)

	if searchErr != nil {
		log.Warnln(logTag, ": error while searching for docs: ", searchErr.Error())
		return false, searchErr
	}

	hitsFound := searchResponse.Hits.TotalHits.Value
	return hitsFound > 0, nil
}

// getRecentDocumentsWithFilter will get the recent documents by applying
// the passed filters accordingly.
func (es recentDocumentsElasticsearch) getRecentDocumentsWithFilter(ctx context.Context, params QueryParams, userId string, indicesPassed []string) ([]byte, error) {
	mustArray := make([]es7.Query, 0)

	// Convert the values of `from` and `to` to epoch
	fromAsTime, parseErr := time.Parse(time.RFC3339, params.From)
	if parseErr != nil {
		return nil, fmt.Errorf("error while parsing `from` into time to convert to epoch: %s", parseErr.Error())
	}
	fromAsInt := fromAsTime.Unix()

	toAsTime, toParseErr := time.Parse(time.RFC3339, params.To)
	if toParseErr != nil {
		return nil, fmt.Errorf("error while parsing `to` into time to convert to epoch: %s", toParseErr.Error())
	}
	toAsInt := toAsTime.Unix()

	// Add the `from` and `to` filters.
	if userId != "" {
		mustArray = append(mustArray, es7.NewExistsQuery(fmt.Sprintf("users.%s", userId)))
		mustArray = append(mustArray, es7.NewRangeQuery(fmt.Sprintf("users.%s", userId)).From(fromAsInt).To(toAsInt))
	}

	// If indexes are present, add them in the filter as well
	for _, index := range indicesPassed {
		mustArray = append(mustArray, es7.NewTermQuery("index.keyword", index))
	}

	finalQuery := es7.NewBoolQuery().Must(mustArray...)
	searchQuery := util.GetClient7().Search(es.index).Query(finalQuery).Size(params.Size)

	// If the userId is passed, sort on the timestamp
	if userId != "" {
		searchQuery = searchQuery.Sort(fmt.Sprintf("users.%s", userId), false)
	}

	// Make the call and parse the results now
	results, searchErr := searchQuery.Do(ctx)
	if searchErr != nil {
		errMsg := fmt.Sprint("Error while searching for recent documents: ", searchErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	responsesToReturn := make([]map[string]interface{}, 0)
	for hitIndex, hit := range results.Hits.Hits {
		sourceAsMap := make(map[string]interface{})
		unmarshalErr := json.Unmarshal(hit.Source, &sourceAsMap)
		if unmarshalErr != nil {
			log.Warnln(logTag, ": error while unmarshalling hit at index: ", hitIndex, " with error: ", unmarshalErr.Error())
			continue
		}

		responsesToReturn = append(responsesToReturn, sourceAsMap)
	}

	responseToReturn := map[string]interface{}{
		"recent_documents": responsesToReturn,
		"total_results":    len(responsesToReturn),
	}

	return json.Marshal(responseToReturn)
}
