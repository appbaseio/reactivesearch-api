package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
	typeName  string
	client    *elastic.Client
}

// NewES initializes the elasticsearch client for the 'analytics' plugin. The function
// is expected to be executed only once, ideally during the initialization of the plugin.
func NewES(url, indexName, typeName, mapping string) (*elasticsearch, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
	}
	ctx := context.Background()

	// Initialize the client
	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v\n", logTag, err)
	}
	es := &elasticsearch{url, indexName, typeName, client}

	// Check if the meta index already exists
	exists, err := client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while checking if index already exists: %v\n",
			logTag, err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, indexName)
		return es, nil
	}

	// Meta index does not exists, create a new one
	_, err = client.CreateIndex(indexName).Body(mapping).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while creating index named %s: %v\n",
			logTag, indexName, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, indexName)
	return es, nil
}

func (es *elasticsearch) indexRecord(docId string, record map[string]interface{}) {
	_, err := es.client.
		Index().
		Index(es.indexName).
		Type(es.typeName).
		BodyJson(record).
		Id(docId).
		Do(context.Background())
	if err != nil {
		log.Printf("%s: error indexing analytics record for id=%s: %v", logTag, docId, err)
		return
	}
}

func (es *elasticsearch) updateRecord(docId string, record map[string]interface{}) {
	_, err := es.client.
		Update().
		Index(es.indexName).
		Type(es.typeName).
		Index(docId).
		Doc(record).
		Do(context.Background())
	if err != nil {
		log.Printf("%s: error updating analytics record for id=%s: %v", logTag, docId, err)
		return
	}
}

func (es *elasticsearch) deleteOldRecords() {
	body := `{ "query": { "range": { "datestamp": { "lt": "now-30d" } } } }`
	ticker := time.NewTicker(24 * time.Hour)
	for range ticker.C {
		_, err := es.client.
			DeleteByQuery().
			Index(es.indexName).
			Type(es.typeName).
			Body(body).
			Do(context.Background())
		if err != nil {
			log.Printf("%s: error deleting old analytics records: %v", logTag, err)
		}
	}
}

// TODO: async??
func (es *elasticsearch) analyticsOverview(from, to string, size int, clickAnalytics bool) ([]byte, error) {
	var overview []map[string]interface{}

	popularSearches, err := es.popularSearches(from, to, size, clickAnalytics)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		overview = append(overview, map[string]interface{}{
			"popular_searches": []interface{}{},
		})
	} else {
		overview = append(overview, popularSearches)
	}

	noResultsSearches, err := es.noResultsSearches(from, to, size)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		overview = append(overview, map[string]interface{}{
			"no_results_searches": []interface{}{},
		})
	} else {
		overview = append(overview, noResultsSearches)
	}

	searchHistogram, err := es.searchHistogram(from, to, size)
	if err != nil {
		log.Printf("%s: %v", err, err)
		overview = append(overview, map[string]interface{}{
			"search_volume": []interface{}{},
		})
	} else {
		overview = append(overview, searchHistogram)
	}

	return json.Marshal(overview)
}

// TODO: async??
func (es *elasticsearch) advancedAnalytics(from, to string, size int, clickAnalytics bool) ([]byte, error) {
	var advancedAnalytics []map[string]interface{}

	popularSearches, err := es.popularSearches(from, to, size, clickAnalytics)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		advancedAnalytics = append(advancedAnalytics, map[string]interface{}{
			"popular_searches": []interface{}{},
		})
	} else {
		advancedAnalytics = append(advancedAnalytics, popularSearches)
	}

	popularResults, err := es.popularResults(from, to, size, clickAnalytics)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		advancedAnalytics = append(advancedAnalytics, map[string]interface{}{
			"popular_results": []interface{}{},
		})
	} else {
		advancedAnalytics = append(advancedAnalytics, popularResults)
	}

	popularFilters, err := es.popularFilters(from, to, size, clickAnalytics)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		advancedAnalytics = append(advancedAnalytics, map[string]interface{}{
			"popular_filters": []interface{}{},
		})
	} else {
		advancedAnalytics = append(advancedAnalytics, popularFilters)
	}

	noResultsSearches, err := es.noResultsSearches(from, to, size)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		advancedAnalytics = append(advancedAnalytics, map[string]interface{}{
			"no_results_searches": []interface{}{},
		})
	} else {
		advancedAnalytics = append(advancedAnalytics, noResultsSearches)
	}

	searchHistogram, err := es.searchHistogram(from, to, size)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		advancedAnalytics = append(advancedAnalytics, map[string]interface{}{
			"search_volume": []interface{}{},
		})
	} else {
		advancedAnalytics = append(advancedAnalytics, searchHistogram)
	}

	return json.Marshal(advancedAnalytics)
}

func (es *elasticsearch) popularSearches(from, to string, size int, clickAnalytics bool) (map[string]interface{}, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	aggr := elastic.NewTermsAggregation().
		Field("search_query.keyword").
		OrderByCountDesc()

	if clickAnalytics {
		clickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_searches_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular searches response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_searches_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation result from 'popular_searches_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount

		if clickAnalytics {
			newBucket = addClickAnalytics(bucket, bucket.DocCount, newBucket)
		}

		buckets = append(buckets, newBucket)
	}

	popularSearches := make(map[string]interface{})
	if buckets == nil {
		popularSearches["popular_searches"] = []interface{}{}
	} else {
		popularSearches["popular_searches"] = buckets
	}

	return popularSearches, nil
}

func (es *elasticsearch) popularSearchesRaw(from, to string, size int, clickAnalytics bool) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	aggr := elastic.NewTermsAggregation().
		Field("search_query.keyword").
		OrderByCountDesc()

	if clickAnalytics {
		clickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_searches_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular searches response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_searches_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation result from 'popular_searches_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount

		if clickAnalytics {
			newBucket = addClickAnalytics(bucket, bucket.DocCount, newBucket)
		}

		buckets = append(buckets, newBucket)
	}

	popularSearches := make(map[string]interface{})
	if buckets == nil {
		popularSearches["popular_searches"] = []interface{}{}
	} else {
		popularSearches["popular_searches"] = buckets
	}

	return json.Marshal(popularSearches)
}

func (es *elasticsearch) noResultsSearchesRaw(from, to string, size int) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	zeroHits := elastic.NewTermQuery("total_hits", 0)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration, zeroHits)

	aggr := elastic.NewTermsAggregation().
		Field("search_query.keyword").
		OrderByCountDesc()

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("no_results_searches_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch no results searches from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("no_results_searches_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation result in 'no_results_searches_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	noResultsSearches := make(map[string]interface{})
	if buckets == nil {
		noResultsSearches["no_results_searches"] = []interface{}{}
	} else {
		noResultsSearches["no_results_searches"] = buckets
	}

	return json.Marshal(noResultsSearches)
}

func (es *elasticsearch) noResultsSearches(from, to string, size int) (map[string]interface{}, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	zeroHits := elastic.NewTermQuery("total_hits", 0)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration, zeroHits)

	aggr := elastic.NewTermsAggregation().
		Field("search_query.keyword").
		OrderByCountDesc()

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("no_results_searches_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch no results searches from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("no_results_searches_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation result in 'no_results_searches_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	noResultsSearches := make(map[string]interface{})
	if buckets == nil {
		noResultsSearches["no_results_searches"] = []interface{}{}
	} else {
		noResultsSearches["no_results_searches"] = buckets
	}

	return noResultsSearches, nil
}


func (es *elasticsearch) popularFiltersRaw(from, to string, size int, clickAnalytics bool) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	valueAggr := elastic.NewTermsAggregation().
		Field("search_filters.value.keyword").
		OrderByCountDesc()
	aggr := elastic.NewTermsAggregation().
		Field("search_filters.key.keyword").OrderByCountDesc().
		SubAggregation("values_aggr", valueAggr).OrderByCountDesc()

	if clickAnalytics {
		clickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_filters_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular filters from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_filters_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation result in 'popular_filters_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		valuesAggrResult, found := bucket.Terms("values_aggr")
		if !found {
			log.Printf("%s: unable to find 'values_aggr' in aggregation result", logTag)
			continue
		}
		for _, valueBucket := range valuesAggrResult.Buckets {
			newBucket := make(map[string]interface{})
			newBucket["key"] = bucket.Key
			newBucket["value"] = valueBucket.Key
			newBucket["count"] = valueBucket.DocCount
			if clickAnalytics {
				newBucket = addClickAnalytics(bucket, valueBucket.DocCount, newBucket)
			}
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

func (es *elasticsearch) popularFilters(from, to string, size int, clickAnalytics bool) (map[string]interface{}, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	valueAggr := elastic.NewTermsAggregation().
		Field("search_filters.value.keyword").
		OrderByCountDesc()
	aggr := elastic.NewTermsAggregation().
		Field("search_filters.key.keyword").OrderByCountDesc().
		SubAggregation("values_aggr", valueAggr).OrderByCountDesc()

	if clickAnalytics {
		clickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_filters_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular filters from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_filters_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation result in 'popular_filters_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		valuesAggrResult, found := bucket.Terms("values_aggr")
		if !found {
			log.Printf("%s: unable to find 'values_aggr' in aggregation result", logTag)
			continue
		}
		for _, valueBucket := range valuesAggrResult.Buckets {
			newBucket := make(map[string]interface{})
			newBucket["key"] = bucket.Key
			newBucket["value"] = valueBucket.Key
			newBucket["count"] = valueBucket.DocCount
			if clickAnalytics {
				newBucket = addClickAnalytics(bucket, valueBucket.DocCount, newBucket)
			}
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

	return popularFilters, nil
}

func (es *elasticsearch) popularResultsRaw(from, to string, size int, clickAnalytics bool) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	fetchSourceCtx := elastic.NewFetchSourceContext(true).Include("hits_in_response.source")
	sourceAggr := elastic.NewTopHitsAggregation().
		Size(1).
		FetchSourceContext(fetchSourceCtx)
	aggr := elastic.NewTermsAggregation().
		Field("hits_in_response.id.keyword").
		OrderByCountDesc().
		SubAggregation("source_aggr", sourceAggr)

	if clickAnalytics {
		clickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_results_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular searches response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_results_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation result from 'popular_results_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		if clickAnalytics {
			newBucket = addClickAnalytics(bucket, bucket.DocCount, newBucket)
		}
		// TODO: source aggr?
		buckets = append(buckets, newBucket)
	}

	popularResults := make(map[string]interface{})
	if buckets == nil {
		popularResults["popular_results"] = []interface{}{}
	} else {
		popularResults["popular_results"] = buckets
	}

	return json.Marshal(popularResults)
}

func (es *elasticsearch) popularResults(from, to string, size int, clickAnalytics bool) (map[string]interface{}, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	fetchSourceCtx := elastic.NewFetchSourceContext(true).Include("hits_in_response.source")
	sourceAggr := elastic.NewTopHitsAggregation().
		Size(1).
		FetchSourceContext(fetchSourceCtx)
	aggr := elastic.NewTermsAggregation().
		Field("hits_in_response.id.keyword").
		OrderByCountDesc().
		SubAggregation("source_aggr", sourceAggr)

	if clickAnalytics {
		clickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_results_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular searches response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_results_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation result from 'popular_results_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		if clickAnalytics {
			newBucket = addClickAnalytics(bucket, bucket.DocCount, newBucket)
		}
		// TODO: source aggr?
		buckets = append(buckets, newBucket)
	}

	popularResults := make(map[string]interface{})
	if buckets == nil {
		popularResults["popular_results"] = []interface{}{}
	} else {
		popularResults["popular_results"] = buckets
	}

	return popularResults, nil
}

func (es *elasticsearch) geoRequestsDistribution(from, to string, size int) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	aggr := elastic.NewTermsAggregation().
		Field("country.keyword").
		OrderByCountDesc()

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("geo_dist_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch request distributions from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("geo_dist_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation result in 'req_dist_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		if bucket.Key.(string) != "" { // TODO: check?
			newBucket := make(map[string]interface{})
			newBucket["key"] = bucket.Key
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

func (es *elasticsearch) latencies(from, to string, size int) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	aggr := elastic.NewHistogramAggregation().
		Field("took").
		Interval(10)

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("latency_aggr", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch latency from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Histogram("latency_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregatio result in 'latency_aggr'")
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

// TODO: TEST??
func clickAnalyticsOnTerms(aggr *elastic.TermsAggregation) {
	clickAggr := elastic.NewTermsAggregation().
		Field("click").
		OrderByCountDesc()

	clickPositionAggr := elastic.NewAvgAggregation().
		Field("click_position")

	conversionAggr := elastic.NewTermsAggregation().
		Field("conversion")

	aggr.
		SubAggregation("click_aggr", clickAggr).
		SubAggregation("click_position_aggr", clickPositionAggr).
		SubAggregation("conversion_aggr", conversionAggr)
}

func (es *elasticsearch) summary(from, to string, size int) ([]byte, error) {
	totalSearches, err := es.totalSearches(from, to)
	if err != nil {
		return nil, err
	}
	totalClicks, err := es.totalClicks(from, to)
	if err != nil {
		return nil, err
	}
	totalConversions, err := es.totalConversions(from, to)
	if err != nil {
		return nil, err
	}

	var avgClickRate, avgConversionRate float64
	if totalSearches == 0 {
		avgClickRate = 0
		avgConversionRate = 0
	} else {
		avgClickRate = totalClicks / totalSearches * 100
		avgConversionRate = totalConversions / totalSearches * 100
	}

	totalSearches, err = strconv.ParseFloat(fmt.Sprintf("%.2f", totalSearches), 64)
	if err != nil {
		return nil, err
	}
	avgClickRate, err = strconv.ParseFloat(fmt.Sprintf("%.2f", avgClickRate), 64)
	if err != nil {
		return nil, err
	}
	avgConversionRate, err = strconv.ParseFloat(fmt.Sprintf("%.2f", avgConversionRate), 64)
	if err != nil {
		return nil, err
	}

	summary := map[string]float64{
		"total_searches":      totalSearches,
		"avg_click_rate":      avgClickRate,
		"avg_conversion_rate": avgConversionRate,
	}

	return json.Marshal(summary)
}

func (es *elasticsearch) totalSearches(from, to string) (float64, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	aggr := elastic.NewValueCountAggregation().Field("indices.keyword")

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("total_searches_aggr", aggr).
		Do(context.Background())
	if err != nil {
		return 0, nil
	}

	aggrResult, found := result.Aggregations.ValueCount("total_searches_aggr")
	if !found {
		return 0, fmt.Errorf("unable to find aggregation result in 'total_searches_aggr'")
	}

	return *aggrResult.Value, nil
}

func (es *elasticsearch) totalConversions(from, to string) (float64, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	convertedSearches := elastic.NewTermQuery("conversion", true)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration, convertedSearches)

	count, err := elastic.NewCountService(es.client).
		Query(query).
		Do(context.Background())
	if err != nil {
		return 0, err
	}

	return float64(count), nil
}

func (es *elasticsearch) totalClicks(from, to string) (float64, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	clicks := elastic.NewTermQuery("click", true)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration, clicks)

	count, err := elastic.NewCountService(es.client).
		Query(query).
		Do(context.Background())
	if err != nil {
		return 0, err
	}

	return float64(count), nil
}

func (es *elasticsearch) searchHistogramRaw(from, to string, size int) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	aggr := elastic.NewDateHistogramAggregation().
		Interval("day").
		Field("datestamp")

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("search_histogram_aggr", aggr).
		Size(0).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch date histogram from es: %v", err)
	}

	aggrResult, found := result.Aggregations.DateHistogram("search_histogram_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation result in 'search_histogram_aggr'")
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

func (es *elasticsearch) searchHistogram(from, to string, size int) (map[string]interface{}, error) {
	duration := elastic.NewRangeQuery("datestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Must(elastic.NewMatchAllQuery()).
		Filter(duration)

	aggr := elastic.NewDateHistogramAggregation().
		Interval("day").
		Field("datestamp")

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("search_histogram_aggr", aggr).
		Size(0).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("unable to fetch date histogram from es: %v", err)
	}

	aggrResult, found := result.Aggregations.DateHistogram("search_histogram_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation result in 'search_histogram_aggr'")
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

	return searchHistogram, nil
}

// TODO: TEST??
func addClickAnalytics(r *elastic.AggregationBucketKeyItem, count int64, newBucket map[string]interface{}) map[string]interface{} {
	// click aggregation
	clickAggrResult, found := r.Terms("click_aggr")
	if found {
		for _, bucket := range clickAggrResult.Buckets {
			if *bucket.KeyAsString == "true" {
				newBucket["clicks"] = bucket.DocCount
			}
		}
		if newBucket["clicks"] == nil {
			newBucket["clicks"] = int64(0)
		}
	} else {
		log.Printf("%s: cannot find click aggregation result in aggregation result", logTag)
	}

	// click position aggregation
	clickPositionAggrResult, found := r.Avg("click_position_aggr")
	if found {
		if clickPositionAggrResult.Value != nil {
			newBucket["click_position"] = clickPositionAggrResult.Value
		} else {
			newBucket["click_position"] = 0 // TODO: default value 0?
		}
	} else {
		log.Printf("%s: cannot find click position aggregation result in aggregation result", logTag)
	}

	// conversion aggregation
	conversionAggrResult, found := r.Terms("conversion_aggr")
	if found {
		var totalConversions int64
		for _, bucket := range conversionAggrResult.Buckets {
			if *bucket.KeyAsString == "true" {
				totalConversions = bucket.DocCount
			}
		}
		if newBucket["clicks"] != nil {
			newBucket["conversion_rate"] = (float64(totalConversions) / float64(count)) * 100
			newBucket["click_rate"] = float64(newBucket["clicks"].(int64)) / float64(count) * 100 // TODO: check cast?
		} else {
			newBucket["conversion_rate"] = 0
			newBucket["click_rate"] = 0
		}
	} else {
		log.Printf("%s: cannot find conversion aggregation result in aggregation result", logTag)
	}

	return newBucket
}
