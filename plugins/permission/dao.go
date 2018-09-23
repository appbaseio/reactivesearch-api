package permission

import (
	"context"
	"fmt"
	"log"

	"github.com/olivere/elastic"
)

type ElasticSearch struct {
	url     string
	index   string
	mapping string
	client  *elastic.Client
}

func NewES(url, index, mapping string) (*ElasticSearch, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
	}
	ctx := context.Background()

	// Initialize the client
	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic client: %v\n", err)
	}
	es := &ElasticSearch{url, index, mapping, client}

	// Check if the meta index already exists
	exists, err := client.IndexExists(index).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v\n", err)
	}
	if exists {
		log.Printf("[INFO] index named '%s' already exists, skipping...", index)
		return es, nil
	}

	// Create a new meta index
	_, err = client.CreateIndex(index).Body(mapping).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v\n", index, err)
	}

	log.Printf("[INFO] successfully created index named '%s'", index)
	return es, nil
}
