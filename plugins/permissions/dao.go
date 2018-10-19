package permissions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
	typeName  string
	mapping   string
	client    *elastic.Client
}

// NewES initializes the elasticsearch client for the 'permissions' plugin. The function
// is expected to be executed only once, ideally during the initialization of the plugin.
func NewES(url, indexName, mapping string) (*elasticsearch, error) {
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
	es := &elasticsearch{url, indexName, "_doc", mapping, client}

	// Check if the meta index already exists
	exists, err := client.IndexExists(indexName).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while checking if index already exists: %v\n", logTag, err)
	}
	if exists {
		log.Printf("%s index named '%s' already exists, skipping...", logTag, indexName)
		return es, nil
	}

	// Create a new meta index
	_, err = client.CreateIndex(indexName).Body(mapping).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while creating index named %s: %v\n", logTag, indexName, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, indexName)
	return es, nil
}

func (es *elasticsearch) getRawPermission(username string) ([]byte, error) {
	resp, err := es.client.Get().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		FetchSource(true).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	src, err := resp.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) putPermission(p permission.Permission) (bool, error) {
	_, err := es.client.Index().
		Index(es.indexName).
		Type(es.typeName).
		Id(p.Username).
		BodyJson(p).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) patchPermission(username string, patch map[string]interface{}) (bool, error) {
	_, err := es.client.Update().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		Doc(patch).
		Do(context.Background())
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (es *elasticsearch) deletePermission(userId string) (bool, error) {
	_, err := es.client.Delete().
		Index(es.indexName).
		Type(es.typeName).
		Id(userId).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) getUserPermissions(userId string) ([]byte, error) {
	resp, err := es.client.Search().
		Index(es.indexName).
		Type(es.typeName).
		Query(elastic.NewTermQuery("user_id", userId)).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	var rawPermissions []*json.RawMessage
	for _, hit := range resp.Hits.Hits {
		rawPermissions = append(rawPermissions, hit.Source)
	}

	raw, err := json.Marshal(rawPermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal slice of raw permissions: %v", err)
	}

	return raw, nil
}
