package users

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/util"
	"gopkg.in/olivere/elastic.v6"
)

type elasticsearch struct {
	url       string
	indexName string
	typeName  string
	client    *elastic.Client
}

func newClient(url, indexName, mapping string) (*elasticsearch, error) {
	ctx := context.Background()

	// Initialize the client
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(util.NewRetrier()),
		elastic.SetSniff(false),
		elastic.SetHttpClient(util.HTTPClient()),
	)

	if err != nil {
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v", logTag, err)
	}
	es := &elasticsearch{url, indexName, "_doc", client}
	defer func() {
		if es != nil {
			if err := es.postMasterUser(); err != nil {
				log.Printf("%s: %v", logTag, err)
			}
		}
	}()

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
	settings := fmt.Sprintf(mapping, nodes, nodes-1)
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

func (es *elasticsearch) postMasterUser() error {
	// Create a master user, if credentials are not provided, we create a default
	// master user. Arc shouldn't be initialized without a root user.
	username, password := os.Getenv("USERNAME"), os.Getenv("PASSWORD")
	if username == "" {
		username, password = "foo", "bar"
	}
	admin, err := user.NewAdmin(username, password)
	if err != nil {
		return fmt.Errorf("%s: error while creating a master user: %v", logTag, err)
	}
	if created, err := es.postUser(context.Background(), *admin); !created || err != nil {
		return fmt.Errorf("%s: error while creating a master user: %v", logTag, err)
	}
	return nil
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

func (es *elasticsearch) getRawUsers(ctx context.Context) ([]byte, error) {
	response, err := es.client.Search().
		Index(es.indexName).
		Type(es.typeName).
		Do(ctx)
	if err != nil {

	}

	var users []*json.RawMessage
	for _, hit := range response.Hits.Hits {
		users = append(users, hit.Source)
	}

	return json.Marshal(users)
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
		Refresh("wait_for").
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
		Refresh("wait_for").
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
		Refresh("wait_for").
		Index(es.indexName).
		Type(es.typeName).
		Id(username).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}
