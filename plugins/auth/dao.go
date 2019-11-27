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
)

type elasticSearch struct {
	url                             string
	userIndex, userType             string
	permissionIndex, permissionType string
}

type publicKey struct {
	PublicKey string `json:"public_key"`
	RoleKey   string `json:"role_key"`
}

func initPlugin(url, userIndex, permissionIndex string) (*elasticSearch, error) {
	// auth only has to establish a connection to es, users, permissions
	// plugin handles the creation of their respective meta indices
	es := &elasticSearch{
		url,
		userIndex, "_doc",
		permissionIndex, "_doc",
	}

	return es, nil
}

func (es *elasticSearch) createIndex(indexName, mapping string) (bool, error) {
	ctx := context.Background()

	// Check if the index already exists
	exists, err := util.GetClient7().IndexExists(indexName).
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
	nodes, err := util.GetTotalNodes()
	if err != nil {
		return false, err
	}
	settings := fmt.Sprintf(mapping, nodes, nodes-1)
	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(indexName).
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
func (es *elasticSearch) savePublicKey(ctx context.Context, indexName string, record publicKey) (interface{}, error) {
	_, err := util.GetClient7().
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
func (es *elasticSearch) getPublicKey(ctx context.Context) (publicKey, error) {
	publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
	if publicKeyIndex == "" {
		publicKeyIndex = defaultPublicKeyEsIndex
	}
	switch util.GetVersion() {
	case 6:
		return es.GetPublicKeyEs6(ctx, publicKeyIndex, publicKeyDocID)
	default:
		return es.GetPublicKeyEs7(ctx, publicKeyIndex, publicKeyDocID)
	}
}

func (es *elasticSearch) getCredential(ctx context.Context, username string) (credential.AuthCredential, error) {
	switch util.GetVersion() {
	case 6:
		return es.GetCredentialEs6(ctx, username)
	default:
		return es.GetCredentialEs7(ctx, username)
	}
}

func (es *elasticSearch) putUser(ctx context.Context, u user.User) (bool, error) {
	_, err := util.GetClient7().Index().
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

func (es *elasticSearch) getUser(ctx context.Context, username string) (*user.User, error) {
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

func (es *elasticSearch) getRawUser(ctx context.Context, username string) ([]byte, error) {
	data, err := util.GetClient7().Get().
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

func (es *elasticSearch) putPermission(ctx context.Context, p permission.Permission) (bool, error) {
	_, err := util.GetClient7().Index().
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

func (es *elasticSearch) getPermission(ctx context.Context, username string) (*permission.Permission, error) {
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

func (es *elasticSearch) getRawPermission(ctx context.Context, username string) ([]byte, error) {
	resp, err := util.GetClient7().Get().
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

func (es *elasticSearch) getRolePermission(ctx context.Context, role string) (*permission.Permission, error) {
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

func (es *elasticSearch) getRawRolePermission(ctx context.Context, role string) ([]byte, error) {
	switch util.GetVersion() {
	case 6:
		return es.GetRawRolePermissionEs6(ctx, role)
	default:
		return es.GetRawRolePermissionEs7(ctx, role)
	}
}
