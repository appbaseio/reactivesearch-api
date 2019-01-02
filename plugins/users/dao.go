package users

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/arc/model/user"
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
	exists, err := client.IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while checking if index already exists: %v",
			logTag, err)
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
	_, err = client.CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while creating index named %s: %v",
			logTag, indexName, err)
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

func (es *elasticsearch) getUser(ctx context.Context, username string) (*user.User, error) {
	raw, err := es.getRawUser(ctx, username)
	if err != nil {
		return nil, err
	}

	var u user.User
	err = json.Unmarshal(raw, &u)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (es *elasticsearch) getRawUser(ctx context.Context, username string) ([]byte, error) {
	response, err := es.client.Get().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	src, err := response.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) postUser(ctx context.Context, u user.User) (bool, error) {
	_, err := es.client.Index().
		Index(es.indexName).
		Type(es.typeName).
		Id(u.Username).
		BodyJson(u).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) patchUser(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error) {
	response, err := es.client.Update().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		Doc(patch).
		Fields("_source").
		Do(ctx)
	if err != nil {
		return nil, err
	}

	src, err := response.GetResult.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) deleteUser(ctx context.Context, username string) (bool, error) {
	_, err := es.client.Delete().
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}
