package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
	es6 "gopkg.in/olivere/elastic.v6"
)

type ElasticSearch struct {
	url                             string
	userIndex, userType             string
	permissionIndex, permissionType string
	client7                         *es7.Client
	client6                         *es6.Client
}

type PublicKey struct {
	PublicKey string `json:"public_key"`
	RoleKey   string `json:"role_key"`
}

func newClient(url, userIndex, permissionIndex string) (*ElasticSearch, error) {
	// auth only has to establish a connection to es, users, permissions
	// plugin handles the creation of their respective meta indices
	es := &ElasticSearch{
		url,
		userIndex, "_doc",
		permissionIndex, "_doc",
		util.Client7,
		util.Client6,
	}

	return es, nil
}

func (es *ElasticSearch) createIndex(indexName, mapping string) (bool, error) {
	ctx := context.Background()

	// Check if the index already exists
	exists, err := es.client7.IndexExists(indexName).
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
	_, err = es.client7.CreateIndex(indexName).
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
func (es *ElasticSearch) savePublicKey(ctx context.Context, indexName string, record PublicKey) (interface{}, error) {
	_, err := es.client7.
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
func (es *ElasticSearch) getPublicKey(ctx context.Context) (PublicKey, error) {
	publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
	if publicKeyIndex == "" {
		publicKeyIndex = defaultPublicKeyEsIndex
	}
	if util.IsCurrentVersionES6 {
		return es.GetPublicKeyEs6(ctx, publicKeyIndex, publicKeyDocID)
	}
	return es.GetPublicKeyEs7(ctx, publicKeyIndex, publicKeyDocID)
}

func (es *ElasticSearch) getTotalNodes() (int, error) {
	if util.IsCurrentVersionES6 {
		return util.GetTotalNodesEs6(es.client6)
	}
	return util.GetTotalNodesEs7(es.client7)
}

func (es *ElasticSearch) getCredential(ctx context.Context, username string) (credential.AuthCredential, error) {
	if util.IsCurrentVersionES6 {
		return es.GetCredentialEs6(ctx, username)
	}
	return es.GetCredentialEs7(ctx, username)
}

func (es *ElasticSearch) putUser(ctx context.Context, u user.User) (bool, error) {
	_, err := es.client7.Index().
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

func (es *ElasticSearch) getUser(ctx context.Context, username string) (*user.User, error) {
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

func (es *ElasticSearch) getRawUser(ctx context.Context, username string) ([]byte, error) {
	data, err := es.client7.Get().
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

func (es *ElasticSearch) putPermission(ctx context.Context, p permission.Permission) (bool, error) {
	_, err := es.client7.Index().
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

func (es *ElasticSearch) getPermission(ctx context.Context, username string) (*permission.Permission, error) {
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

func (es *ElasticSearch) getRawPermission(ctx context.Context, username string) ([]byte, error) {
	resp, err := es.client7.Get().
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

func (es *ElasticSearch) getRolePermission(ctx context.Context, role string) (*permission.Permission, error) {
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

func (es *ElasticSearch) getRawRolePermission(ctx context.Context, role string) ([]byte, error) {
	if util.IsCurrentVersionES6 {
		return es.GetRawRolePermissionEs6(ctx, role)
	}
	return es.GetRawRolePermissionEs7(ctx, role)
}
