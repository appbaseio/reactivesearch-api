package logs

import (
	"context"
	"fmt"
	"log"

	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
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
	es := &elasticsearch{url, indexName, client}

	// Check if meta index already exists
	exists, err := client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named \"%s\" already exists, skipping ...\n", logTag, indexName)
		return es, nil
	}

	// Meta index doesn't exist, create one
	_, err = client.CreateIndex(indexName).Body(mapping).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named \"%s\"", indexName)
	}

	log.Printf("%s: successfully created index name \"%s\"", logTag, indexName)
	return es, nil
}

func (es *elasticsearch) indexRecord(record record) {
	_, err := es.client.
		Index().
		Index(es.indexName).
		Type("_doc").
		BodyJson(record).
		Do(context.Background())
	if err != nil {
		log.Printf("%s: error indexing logs record: %v", logTag, err)
		return
	}
}
