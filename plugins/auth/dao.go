package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/util"
	"github.com/olivere/elastic/v7"
)

type elasticsearch struct {
	url                             string
	userIndex, userType             string
	permissionIndex, permissionType string
	client                          *elastic.Client
}

type publicKey struct {
	PublicKey string `json:"public_key"`
	RoleKey   string `json:"role_key"`
}

func newClient(url, userIndex, permissionIndex string) (*elasticsearch, error) {
	// auth only has to establish a connection to es, users, permissions
	// plugin handles the creation of their respective meta indices
	client, err := elastic.NewClient(
		elastic.SetURL(url),
		elastic.SetRetrier(util.NewRetrier()),
		elastic.SetSniff(false),
		elastic.SetHttpClient(util.HTTPClient()),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v", logTag, err)
	}

	es := &elasticsearch{
		url,
		userIndex, "_doc",
		permissionIndex, "_doc",
		client,
	}

	return es, nil
}

func (es *elasticsearch) createIndex(indexName, mapping string) (bool, error) {
	ctx := context.Background()

	// Check if the index already exists
	exists, err := es.client.IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: error while checking if index already exists: %v",
			logTag, err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, indexName)
		return true, nil
	}

	// set the number_of_replicas to (nodes-1)
	nodes, err := es.getTotalNodes()
	if err != nil {
		return false, err
	}
	settings := fmt.Sprintf(mapping, nodes, nodes-1)
	// Meta index does not exists, create a new one
	_, err = es.client.CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: error while creating index named %s: %v",
			logTag, indexName, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, indexName)
	return true, nil
}

// Create or update the public key
func (es *elasticsearch) savePublicKey(ctx context.Context, indexName string, record publicKey) (interface{}, error) {
	_, err := es.client.
		Index().
		Index(indexName).
		BodyJson(record).
		Id(publicKeyDocID).
		Do(ctx)
	if err != nil {
		log.Printf("%s: error indexing public key record", logTag)
		return false, err
	}

	return true, nil
}

// Get the public key
func (es *elasticsearch) getPublicKey(ctx context.Context) (publicKey, error) {
	var record = publicKey{}
	publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
	if publicKeyIndex == "" {
		publicKeyIndex = defaultPublicKeyEsIndex
	}
	response, err := es.client.Get().
		Index(publicKeyIndex).
		Id(publicKeyDocID).
		Do(ctx)
	if response == nil {
		return record, errors.New("public key record not found")
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Printf("%s: error retrieving publickey record", logTag)
		return record, err
	}
	return record, nil
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

func (es *elasticsearch) getCredential(ctx context.Context, username string) (credential.AuthCredential, error) {
	matchUsername := elastic.NewTermQuery("username.keyword", username)

	response, err := es.client.Search().
		Index(es.userIndex, es.permissionIndex).
		Query(matchUsername).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	if len(response.Hits.Hits) > 1 {
		return nil, fmt.Errorf(`more than one result for "username"="%s"`, username)
	}

	// there should be either 0 or 1 hit
	var obj credential.AuthCredential
	for _, hit := range response.Hits.Hits {
		if hit.Index == es.userIndex {
			var u user.User
			if hit.Source != nil {
				err := json.Unmarshal(hit.Source, &u)
				if err != nil {
					return nil, err
				}
				obj = &u
			}
		} else if hit.Index == es.permissionIndex {
			var p permission.Permission

			// unmarshal into permission
			err := json.Unmarshal(hit.Source, &p)
			if err != nil {
				return nil, err
			}

			obj = &p
		}
	}

	return obj, nil
}

func (es *elasticsearch) putUser(ctx context.Context, u user.User) (bool, error) {
	_, err := es.client.Index().
		Index(es.userIndex).
		Type(es.userType).
		Id(u.Username).
		BodyJson(u).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) getUser(ctx context.Context, username string) (*user.User, error) {
	data, err := es.getRawUser(ctx, username)
	if err != nil {
		return nil, err
	}
	var u user.User
	err = json.Unmarshal(data, &u)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (es *elasticsearch) getRawUser(ctx context.Context, username string) ([]byte, error) {
	data, err := es.client.Get().
		Index(es.userIndex).
		Type(es.userType).
		Id(username).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	src, err := data.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) putPermission(ctx context.Context, p permission.Permission) (bool, error) {
	_, err := es.client.Index().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Id(p.Username).
		BodyJson(p).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) getPermission(ctx context.Context, username string) (*permission.Permission, error) {
	data, err := es.getRawPermission(ctx, username)
	if err != nil {
		return nil, err
	}

	var p permission.Permission
	err = json.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (es *elasticsearch) getRawPermission(ctx context.Context, username string) ([]byte, error) {
	resp, err := es.client.Get().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Id(username).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	src, err := resp.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) getRolePermission(ctx context.Context, role string) (*permission.Permission, error) {
	data, err := es.getRawRolePermission(ctx, role)
	if err != nil {
		return nil, err
	}

	var p permission.Permission
	err = json.Unmarshal(data, &p)
	if err != nil {
		return nil, err
	}

	return &p, nil
}

func (es *elasticsearch) getRawRolePermission(ctx context.Context, role string) ([]byte, error) {
	resp, err := es.client.Search().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Query(elastic.NewTermQuery("role", role)).
		Size(1).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	for _, hit := range resp.Hits.Hits {
		src, err := json.Marshal(hit.Source)
		if err == nil {
			return src, nil
		}
	}
	return nil, nil
}
