package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/appbaseio-confidential/arc/util"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
	typeName  string
	client    *elastic.Client
}

func newClient(url, indexName, mapping string) (*elasticsearch, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
	}
	ctx := context.Background()

	// Initialize the client
	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic client: %v", err)
	}
	es := &elasticsearch{url, indexName, "_doc", client}

	// Check if the meta index already exists
	exists, err := client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, indexName)
		return es, nil
	}

	// set the number_of_replicas to (nodes-1)
	nodes, err := es.getTotalNodes()
	if err != nil {
		return nil, err
	}
	settings := fmt.Sprintf(mapping, nodes-1)

	// Meta index does not exists, create a new one
	_, err = client.CreateIndex(indexName).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", indexName, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, indexName)
	return es, nil
}

func (es *elasticsearch) getTotalNodes() (int, error) {
	response, err := es.client.NodesInfo().
		Metric("nodes").
		Do(context.Background())
	if err != nil {
		return -1, err
	}

	return len(response.Nodes), nil
}

func (es *elasticsearch) indexRecord(ctx context.Context, docID string, record map[string]interface{}) {
	_, err := es.client.
		Index().
		Index(es.indexName).
		Type(es.typeName).
		BodyJson(record).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Printf("%s: error indexing analytics record for id=%s: %v", logTag, docID, err)
		return
	}
}

func (es *elasticsearch) updateRecord(ctx context.Context, docID string, record map[string]interface{}) {
	_, err := es.client.
		Update().
		Index(es.indexName).
		Type(es.typeName).
		Index(docID).
		Doc(record).
		Do(ctx)
	if err != nil {
		log.Printf("%s: error updating analytics record for id=%s: %v", logTag, docID, err)
		return
	}
}

func (es *elasticsearch) deleteOldRecords() {
	body := `{ "query": { "range": { "timestamp": { "lt": "now-30d" } } } }`
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

func (es *elasticsearch) analyticsOverview(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error) {
	var wg sync.WaitGroup
	out := make(chan interface{})

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularSearches, err := es.popularSearches(ctx, from, to, size, clickAnalytics, indices...)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			out <- map[string]interface{}{
				"popular_searches": []interface{}{},
			}
		} else {
			out <- popularSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		noResultsSearches, err := es.noResultSearches(ctx, from, to, size, indices...)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			out <- map[string]interface{}{
				"no_results_searches": []interface{}{},
			}
		} else {
			out <- noResultsSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		searchHistogram, err := es.searchHistogram(ctx, from, to, size, indices...)
		if err != nil {
			log.Printf("%s: %v", err, err)
			out <- map[string]interface{}{
				"search_volume": []interface{}{},
			}
		} else {
			out <- searchHistogram
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var overview []interface{}
	for result := range out {
		overview = append(overview, result)
	}

	return json.Marshal(overview)
}

func (es *elasticsearch) advancedAnalytics(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error) {
	var wg sync.WaitGroup
	out := make(chan interface{})

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularSearches, err := es.popularSearches(ctx, from, to, size, clickAnalytics, indices...)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			out <- map[string]interface{}{
				"popular_searches": []interface{}{},
			}
		} else {
			out <- popularSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularResults, err := es.popularResults(ctx, from, to, size, clickAnalytics, indices...)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			out <- map[string]interface{}{
				"popular_results": []interface{}{},
			}
		} else {
			out <- popularResults
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		popularFilters, err := es.popularFilters(ctx, from, to, size, clickAnalytics, indices...)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			out <- map[string]interface{}{
				"popular_filters": []interface{}{},
			}
		} else {
			out <- popularFilters
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		noResultsSearches, err := es.noResultSearches(ctx, from, to, size, indices...)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			out <- map[string]interface{}{
				"no_results_searches": []interface{}{},
			}
		} else {
			out <- noResultsSearches
		}
	}(out)

	wg.Add(1)
	go func(out chan<- interface{}) {
		defer wg.Done()
		searchHistogram, err := es.searchHistogram(ctx, from, to, size, indices...)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			out <- map[string]interface{}{
				"search_volume": []interface{}{},
			}
		} else {
			out <- searchHistogram
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var advancedAnalytics []interface{}
	for result := range out {
		advancedAnalytics = append(advancedAnalytics, result)
	}

	return json.Marshal(advancedAnalytics)
}

func (es *elasticsearch) popularSearches(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) (interface{}, error) {
	raw, err := es.popularSearchesRaw(ctx, from, to, size, clickAnalytics, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		PopularSearches []map[string]interface{} `json:"popular_searches"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) popularSearchesRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	aggr := elastic.NewTermsAggregation().
		Field("search_query.keyword").
		OrderByCountDesc()

	if clickAnalytics {
		applyClickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_searches_aggr", aggr).
		Size(size).
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

func (es *elasticsearch) noResultSearches(ctx context.Context, from, to string, size int, indices ...string) (interface{}, error) {
	raw, err := es.noResultSearchesRaw(ctx, from, to, size, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		NoResultsSearches []map[string]interface{} `json:"no_results_searches"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) noResultSearchesRaw(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	zeroHits := elastic.NewTermQuery("total_hits", 0)

	query := elastic.NewBoolQuery().Filter(duration, zeroHits)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	aggr := elastic.NewTermsAggregation().
		Field("search_query.keyword").
		OrderByCountDesc()

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("no_results_searches_aggr", aggr).
		Size(size).
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

func (es *elasticsearch) popularFilters(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) (interface{}, error) {
	raw, err := es.popularFiltersRaw(ctx, from, to, size, clickAnalytics, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		PopularFilters []map[string]interface{} `json:"popular_filters"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) popularFiltersRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	valueAggr := elastic.NewTermsAggregation().
		Field("search_filters.value.keyword").
		OrderByCountDesc()
	aggr := elastic.NewTermsAggregation().
		Field("search_filters.key.keyword").OrderByCountDesc().
		SubAggregation("values_aggr", valueAggr).OrderByCountDesc()

	if clickAnalytics {
		applyClickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_filters_aggr", aggr).
		Size(size).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular filters from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_filters_aggr")
	if !found {
		return nil, fmt.Errorf("unable to find aggregation value in 'popular_filters_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		valuesAggrResult, found := bucket.Terms("values_aggr")
		if !found {
			log.Printf("%s: unable to find 'values_aggr' in aggregation value", logTag)
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

func (es *elasticsearch) popularResults(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) (interface{}, error) {
	raw, err := es.popularResultsRaw(ctx, from, to, size, clickAnalytics, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		PopularResults []map[string]interface{} `json:"popular_results"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) popularResultsRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	sourceAggr := elastic.NewTermsAggregation().
		Field("hits_in_response.source.keyword").
		OrderByCountDesc()
	aggr := elastic.NewTermsAggregation().
		Field("hits_in_response.id.keyword").
		OrderByCountDesc().
		SubAggregation("source_aggr", sourceAggr)

	if clickAnalytics {
		applyClickAnalyticsOnTerms(aggr)
	}

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("popular_results_aggr", aggr).
		Size(size).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch popular searches response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("popular_results_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'popular_results_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		sourceAggrResult, found := bucket.Terms("source_aggr")
		if !found {
			log.Printf("%s: unable to find 'source_aggr' in aggregation value", logTag)
			continue
		}
		for _, sourceBucket := range sourceAggrResult.Buckets {
			newBucket := make(map[string]interface{})
			newBucket["key"] = bucket.Key
			newBucket["source"] = sourceBucket.Key
			newBucket["count"] = sourceBucket.DocCount
			if clickAnalytics {
				newBucket = addClickAnalytics(bucket, sourceBucket.DocCount, newBucket)
			}
			buckets = append(buckets, newBucket)
		}
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

func (es *elasticsearch) geoRequestsDistribution(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	aggr := elastic.NewTermsAggregation().
		Field("country.keyword").
		OrderByCountDesc()

	result, err := es.client.Search(es.indexName).
		Query(query).
		Aggregation("geo_dist_aggr", aggr).
		Size(size).
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
			log.Printf("%s: invalid key type %T received for country name", logTag, bucket.Key)
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

func (es *elasticsearch) latencies(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	aggr := elastic.NewHistogramAggregation().
		Field("took").
		Interval(10)

	result, err := es.client.Search(es.indexName).
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

func (es *elasticsearch) summary(ctx context.Context, from, to string, indices ...string) ([]byte, error) {
	type result struct {
		field string
		value float64
		err   error
	}

	var wg sync.WaitGroup
	out := make(chan result)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalSearches, err := es.totalSearches(ctx, from, to, indices...)
		if err != nil {
			out <- result{
				field: "total_searches",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_searches",
				value: totalSearches,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalClicks, err := es.totalClicks(ctx, from, to, indices...)
		if err != nil {
			out <- result{
				field: "total_clicks",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_clicks",
				value: totalClicks,
			}
		}
	}(out)

	wg.Add(1)
	go func(out chan<- result) {
		defer wg.Done()
		totalConversions, err := es.totalConversions(ctx, from, to, indices...)
		if err != nil {
			out <- result{
				field: "total_conversions",
				err:   err,
			}
		} else {
			out <- result{
				field: "total_conversions",
				value: totalConversions,
			}
		}
	}(out)

	go func() {
		wg.Wait()
		close(out)
	}()

	var totalSearches, totalClicks, totalConversions float64
	for result := range out {
		if result.err != nil {
			return nil, fmt.Errorf(`cannot fetch value for "%s"`, result.field)
		}
		switch result.field {
		case "total_searches":
			totalSearches = result.value
		case "total_clicks":
			totalClicks = result.value
		case "total_conversions":
			totalConversions = result.value
		default:
			return nil, fmt.Errorf(`illegal field "%s" encountered`, result.field)
		}
	}

	var avgClickRate, avgConversionRate float64
	if totalSearches != 0 {
		avgClickRate = totalClicks / totalSearches * 100
		avgConversionRate = totalConversions / totalSearches * 100
	}

	summary := map[string]map[string]float64{
		"summary": {
			"total_searches":      util.WithPrecision(totalSearches, 2),
			"avg_click_rate":      util.WithPrecision(avgClickRate, 2),
			"avg_conversion_rate": util.WithPrecision(avgConversionRate, 2),
		},
	}

	return json.Marshal(summary)
}

func (es *elasticsearch) totalSearches(ctx context.Context, from, to string, indices ...string) (float64, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	aggr := elastic.NewValueCountAggregation().Field("indices.keyword")

	result, err := es.client.Search(es.indexName).
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

func (es *elasticsearch) totalConversions(ctx context.Context, from, to string, indices ...string) (float64, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	convertedSearches := elastic.NewTermQuery("conversion", true)

	query := elastic.NewBoolQuery().Filter(duration, convertedSearches)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	count, err := elastic.NewCountService(es.client).
		Query(query).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	return float64(count), nil
}

func (es *elasticsearch) totalClicks(ctx context.Context, from, to string, indices ...string) (float64, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	clicks := elastic.NewTermQuery("click", true)

	query := elastic.NewBoolQuery().Filter(duration, clicks)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	count, err := elastic.NewCountService(es.client).
		Query(query).
		Do(ctx)
	if err != nil {
		return 0, err
	}

	return float64(count), nil
}

func (es *elasticsearch) searchHistogram(ctx context.Context, from, to string, size int, indices ...string) (interface{}, error) {
	raw, err := es.searchHistogramRaw(ctx, from, to, size, indices...)
	if err != nil {
		return []interface{}{}, err
	}

	var response struct {
		SearchHistogram []map[string]interface{} `json:"search_histogram"`
	}
	err = json.Unmarshal(raw, &response)
	if err != nil {
		return []interface{}{}, err
	}

	return response, nil
}

func (es *elasticsearch) searchHistogramRaw(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	aggr := elastic.NewDateHistogramAggregation().
		Interval("day").
		Field("timestamp")

	result, err := es.client.Search(es.indexName).
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

func (es *elasticsearch) getRequestDistribution(ctx context.Context, from, to, interval string, size int, indices ...string) ([]byte, error) {
	duration := elastic.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := elastic.NewBoolQuery().
		Filter(duration)

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	subAggr := elastic.NewTermsAggregation().
		Field("response.code").
		OrderByCountDesc()
	aggr := elastic.NewDateHistogramAggregation().
		Interval(interval).
		Field("timestamp").
		SubAggregation("responses_with_code_aggr", subAggr)

	// TODO: need a solution for maintaining multiple index urls
	result, err := es.client.Search("logs").
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
			log.Printf("%s: unable to find 'responses_with_code_aggr' in aggregation value", logTag)
			continue
		}
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["key_as_string"] = bucket.KeyAsString
		newBucket["count"] = bucket.DocCount

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

// applyClickAnalyticsOnTerms is a mutator that applies aggregations
// for click analytics on the given terms aggregation.
func applyClickAnalyticsOnTerms(aggr *elastic.TermsAggregation) {
	clickAggr := elastic.NewTermsAggregation().
		Field("click").
		OrderByCountDesc()

	clickPositionAggr := elastic.NewAvgAggregation().
		Field("click_position")

	conversionAggr := elastic.NewTermsAggregation().
		Field("conversion")

	aggr.SubAggregation("click_aggr", clickAggr).
		SubAggregation("click_position_aggr", clickPositionAggr).
		SubAggregation("conversion_aggr", conversionAggr)
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
		log.Printf("%s: cannot find click aggregation value in aggregation value", logTag)
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
		log.Printf("%s: cannot find click position aggregation value in aggregation value", logTag)
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
		log.Printf("%s: cannot find conversion aggregation value in aggregation value", logTag)
	}

	return newBucket
}
