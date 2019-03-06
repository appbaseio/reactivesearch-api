package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/appbaseio-confidential/arc/util"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
	client    *elastic.Client
}

func newClient(url, indexName, config string) (*elasticsearch, error) {
	ctx := context.Background()

	// Initialize the client
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(util.NewRetrier()),
		elastic.SetSniff(false),
	)
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic client: %v", err)
	}
	es := &elasticsearch{url, indexName, client}

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

func (es *elasticsearch) getRawLogs(ctx context.Context, from, size string, indices ...string) ([]byte, error) {
	offset, err := strconv.Atoi(from)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "from"`, from)
	}
	s, err := strconv.Atoi(size)
	if err != nil {
		return nil, fmt.Errorf(`invalid value "%v" for query param "size"`, size)
	}

	response, err := es.client.Search(es.indexName).
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
