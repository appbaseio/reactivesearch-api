package users

import (
	"context"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/arc/internal/types/user"
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
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v", logTag, err)
	}
	es := &elasticsearch{url, indexName, "_doc", client}

	// Check if the meta index already exists
	exists, err := client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while checking if index already exists: %v",
			logTag, err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, indexName)
		return es, nil
	}

	// Meta index does not exists, create a new one
	_, err = client.CreateIndex(indexName).Body(mapping).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while creating index named %s: %v",
			logTag, indexName, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, indexName)
	return es, nil
}

func (es *elasticsearch) getRawUser(userID string) ([]byte, error) {
	data, err := es.client.Get().
		Index(es.indexName).
		Type(es.typeName).
		Id(userID).
		FetchSource(true).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	src, err := data.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) postUser(u user.User) (bool, error) {
	_, err := es.client.Index().
		Index(es.indexName).
		Type(es.typeName).
		Id(u.UserId).
		BodyJson(u).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) patchUser(userID string, patch map[string]interface{}) (bool, error) {
	_, err := es.client.Update().
		Index(es.indexName).
		Type(es.typeName).
		Id(userID).
		Doc(patch).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) deleteUser(userID string) (bool, error) {
	_, err := es.client.Delete().
		Index(es.indexName).
		Type(es.typeName).
		Id(userID).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}
