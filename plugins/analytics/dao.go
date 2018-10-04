package analytics

import (
	"context"
	"fmt"
	"log"
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
