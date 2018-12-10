package permissions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url       string
	indexName string
	typeName  string
	mapping   string
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
	es := &elasticsearch{url, indexName, "_doc", mapping, client}

	// Check if the meta index already exists
	exists, err := client.IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while checking if index already exists: %v", logTag, err)
	}
	if exists {
		log.Printf("%s index named '%s' already exists, skipping...", logTag, indexName)
		return es, nil
	}

	// set number_of_replicas to (nodes-1)
	nodes, err := es.getTotalNodes()
	if err != nil {
		return nil, err
	}
	settings := fmt.Sprintf(mapping, (nodes - 1))

	// Create a new meta index
	_, err = client.CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while creating index named %s: %v", logTag, indexName, err)
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

func (es *elasticsearch) getPermission(username string) (*permission.Permission, error) {
	raw, err := es.getRawPermission(username)
	if err != nil {
		return nil, err
	}

	var p permission.Permission
	err = json.Unmarshal(raw, &p)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (es *elasticsearch) getRawPermission(username string) ([]byte, error) {
	response, err := es.client.Get().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		FetchSource(true).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	src, err := response.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) postPermission(p permission.Permission) (bool, error) {
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

func (es *elasticsearch) patchPermission(username string, patch map[string]interface{}) ([]byte, error) {
	response, err := es.client.Update().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		Doc(patch).
		Fields("_source").
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	src, err := response.GetResult.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) deletePermission(username string) (bool, error) {
	_, err := es.client.Delete().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		Do(context.Background())
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) getOwnerPermissions(owner string) ([]byte, error) {
	resp, err := es.client.Search().
		Index(es.indexName).
		Type(es.typeName).
		Query(elastic.NewTermQuery("owner.keyword", owner)).
		Do(context.Background())
	if err != nil {
		return nil, err
	}

	rawPermissions := []*json.RawMessage{}
	for _, hit := range resp.Hits.Hits {
		rawPermissions = append(rawPermissions, hit.Source)
	}

	raw, err := json.Marshal(rawPermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal slice of raw permissions: %v", err)
	}

	return raw, nil
}
