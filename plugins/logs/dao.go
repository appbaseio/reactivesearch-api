package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/appbaseio/arc/util"
	"github.com/olivere/elastic/v7"
	es6 "gopkg.in/olivere/elastic.v6"
)

type elasticsearch struct {
	url       string
	indexName string
	client    *elastic.Client
	client6   *es6.Client
}

const VERSION = 7

func newClient(url, indexName, config string) (*elasticsearch, error) {
	ctx := context.Background()

	// Initialize the ES v7 client
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(util.NewRetrier()),
		elastic.SetSniff(false),
		elastic.SetHttpClient(util.HTTPClient()),
	)
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic v7 client: %v", err)
	}
	// Initialize the ES v6 client
	client6, err := es6.NewClient(
		es6.SetURL(url),
		es6.SetRetrier(util.NewRetrier()),
		es6.SetSniff(false),
		es6.SetHttpClient(util.HTTPClient()),
	)
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic v6 client: %v", err)
	}
	var es *elasticsearch
	if VERSION == 7 {
		es = &elasticsearch{url, indexName, client, nil}
	} else {
		es = &elasticsearch{url, indexName, nil, client6}
	}

	// Check if meta index already exists
	exists, err := client.IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named \"%s\" already exists, skipping ...\n", logTag, indexName)
		return es, nil
	}

	// set number_of_replicas to (nodes-1)
	nodes, err := es.getTotalNodes()
	if err != nil {
		return nil, err
	}
	settings := fmt.Sprintf(config, nodes, nodes-1)

	// Meta index doesn't exist, create one
	_, err = client.CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named \"%s\"", indexName)
	}

	log.Printf("%s: successfully created index name \"%s\"", logTag, indexName)
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

func (es *elasticsearch) indexRecord(ctx context.Context, rec record) {
	bulkIndex := elastic.NewBulkIndexRequest().
		Index(es.indexName).
		Type("_doc").
		Doc(rec)

	_, err := es.client.Bulk().
		Add(bulkIndex).
		Do(ctx)
	if err != nil {
		log.Printf("%s: error indexing log record: %v", logTag, err)
	}
}

func (es *elasticsearch) getRawLogs(ctx context.Context, from, size, filter string, indices ...string) ([]byte, error) {
	if VERSION == 7 {
		return es.getRawLogsES7(ctx, from, size, filter, indices...)
	}
	return es.getRawLogsES6(ctx, from, size, indices...)
}

func (es *elasticsearch) getRawLogsES6(ctx context.Context, from, size string, indices ...string) ([]byte, error) {
	offset, err := strconv.Atoi(from)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "from"`, from)
	}
	s, err := strconv.Atoi(size)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "size"`, size)
	}

	response, err := es.client6.Search(es.indexName).
		From(offset).
		Size(s).
		Sort("timestamp", false).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := []*json.RawMessage{}
	for _, hit := range response.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(*hit.Source, &source)
		if err != nil {
			return nil, err
		}
		rawIndices, ok := source["indices"]
		if !ok {
			log.Printf(`%s: unable to find "indices" in log record\n`, logTag)
		}
		logIndices, err := util.ToStringSlice(rawIndices)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			continue
		}

		if len(indices) == 0 {
			hits = append(hits, hit.Source)
		} else if util.IsSubset(indices, logIndices) {
			hits = append(hits, hit.Source)
		}
	}

	logs := make(map[string]interface{})
	logs["logs"] = hits
	logs["total"] = len(hits)
	logs["took"] = response.TookInMillis

	raw, err := json.Marshal(logs)
	if err != nil {
		return nil, err
	}

	return raw, nil
}

func (es *elasticsearch) getRawLogsES7(ctx context.Context, from, size, filter string, indices ...string) ([]byte, error) {
	fmt.Println("calling get logs: ", from, size, filter, indices)
	offset, err := strconv.Atoi(from)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "from"`, from)
	}
	s, err := strconv.Atoi(size)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "size"`, size)
	}
	query := elastic.NewBoolQuery()
	if filter == "search" {
		filters := elastic.NewTermQuery("category.keyword", "search")
		query.Filter(filters)
	} else if filter == "delete" {
		filters := elastic.NewMatchQuery("request.method.keyword", "DELETE")
		query.Filter(filters)
	} else if filter == "success" {
		filters := elastic.NewRangeQuery("response.code").Gte(200).Lte(299)
		query.Filter(filters)
	} else if filter == "error" {
		filters := elastic.NewRangeQuery("response.code").Gte(400)
		query.Filter(filters)
	} else {
		query.Filter(elastic.NewMatchAllQuery())
	}

	if indices != nil && len(indices) > 0 {
		var indexQueries []elastic.Query
		for _, index := range indices {
			query := elastic.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Must(indexQueries...)
	}

	response, err := es.client.Search(es.indexName).
		Query(query).
		From(offset).
		Size(s).
		Sort("timestamp", false).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := []json.RawMessage{}
	for _, hit := range response.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(hit.Source, &source)
		if err != nil {
			return nil, err
		}
		hits = append(hits, hit.Source)
	}

	logs := make(map[string]interface{})
	logs["logs"] = hits
	logs["total"] = response.Hits.TotalHits.Value
	logs["took"] = response.TookInMillis

	raw, err := json.Marshal(logs)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
