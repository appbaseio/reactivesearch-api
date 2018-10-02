package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/olivere/elastic"
	"github.com/pkg/errors"
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

func (es *elasticsearch) getRawLatency(from, to string, size int) ([]byte, error) {
	duration := elastic.NewRangeQuery("datestamp").From(from).To(to)
	cluster := elastic.NewTermQuery("index_name.keyword", "*")
	query := elastic.NewBoolQuery().Must(elastic.NewMatchAllQuery()).Filter(duration, cluster)
	aggr := elastic.NewHistogramAggregation().Field("took").Interval(10)
	// TODO: should we expect interval as query param?

	response, err := es.client.
		Search(es.indexName).
		Query(query).
		Aggregation("latency", aggr).
		Size(size).
		Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error fetching latency: %v", err)
	}

	result, ok := response.Aggregations.Histogram("latency")
	if !ok {
		return nil, errors.New("unable to fetch latency results from response")
	}

	var latency []map[string]interface{}
	for _, bucket := range result.Buckets {
		newBucket := map[string]interface{}{
			"key":   bucket.Key,
			"count": bucket.DocCount,
		}
		latency = append(latency, newBucket)
	}

	results := make(map[string]interface{})
	if latency == nil {
		results["latency"] = []interface{}{}
	} else {
		results["latency"] = latency
	}

	return json.Marshal(results)
}
