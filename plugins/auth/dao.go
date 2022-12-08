package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/util"
)

type elasticsearch struct {
	userIndex, userType             string
	permissionIndex, permissionType string
}

type publicKey struct {
	PublicKey string `json:"public_key"`
	RoleKey   string `json:"role_key"`
}

func initPlugin(userIndex, permissionIndex string) (*elasticsearch, error) {
	// auth only has to establish a connection to es, users, permissions
	// plugin handles the creation of their respective meta indices
	es := &elasticsearch{
		userIndex, "_doc",
		permissionIndex, "_doc",
	}

	return es, nil
}

func (es *elasticsearch) createIndex(indexName, mapping string) (bool, error) {
	ctx := context.Background()

	// Check if the index already exists
	exists, err := util.GetInternalClient7().IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: error while checking if index already exists: %v",
			logTag, err)
	}
	if exists {
		log.Println(logTag, ": index named", indexName, "already exists, skipping...")
		return true, nil
	}

	replicas := util.GetReplicas()

	settings := fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas)
	// Meta index does not exists, create a new one
	_, err = util.GetInternalClient7().CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: error while creating index named %s: %v",
			logTag, indexName, err)
	}

	log.Println(logTag, ": successfully created index named", indexName)
	return true, nil
}

// Create or update the public key
func (es *elasticsearch) savePublicKey(ctx context.Context, indexName string, record publicKey) (interface{}, error) {

	_, err := util.IndexServiceWithAuth(util.GetInternalClient7().
		Index().
		Index(indexName).
		BodyJson(record).
		Id(publicKeyDocID), ctx).Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error indexing public key record", err)
		return false, err
	}

	return true, nil
}

// Get the public key
func (es *elasticsearch) getPublicKey(ctx context.Context) (publicKey, error) {
	publicKeyIndex := os.Getenv(envPublicKeyEsIndex)
	if publicKeyIndex == "" {
		publicKeyIndex = defaultPublicKeyEsIndex
	}
	return es.getPublicKeyEs7(ctx, publicKeyIndex, publicKeyDocID)
}

func (es *elasticsearch) getCredential(ctx context.Context, username string) (credential.AuthCredential, error) {
	return es.getCredentialEs7(ctx, username)
}

func (es *elasticsearch) putUser(ctx context.Context, u user.User) (bool, error) {
	_, err := util.IndexServiceWithAuth(util.GetInternalClient7().Index().
		Index(es.userIndex).
		Type(es.userType).
		Id(u.Username).
		BodyJson(u), ctx).Do(ctx)

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
	searchQuery := es7.NewTermQuery("_id", username)
	response, err := util.SearchServiceWithAuth(util.GetInternalClient7().Search().
		Index(es.userIndex).
		Type(es.userType).
		FetchSource(true).Query(searchQuery), ctx).Do(ctx)

	if err != nil {
		return nil, err
	}

	if len(response.Hits.Hits) < 1 {
		return nil, fmt.Errorf("no username found for: %s", username)
	}

	data := response.Hits.Hits[0]

	src, err := data.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

func (es *elasticsearch) putPermission(ctx context.Context, p permission.Permission) (bool, error) {

	_, err := util.IndexServiceWithAuth(util.GetInternalClient7().Index().
		Index(es.permissionIndex).
		Type(es.permissionType).
		Id(p.Username).
		BodyJson(p), ctx).Do(ctx)

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

	searchQuery := es7.NewTermQuery("_id", username)
	response, err := util.SearchServiceWithAuth(util.GetInternalClient7().Search().
		Index(es.permissionIndex).
		Type(es.permissionType).
		FetchSource(true).
		Query(searchQuery), ctx).Do(ctx)

	if err != nil {
		return nil, err
	}

	if len(response.Hits.Hits) < 1 {
		return nil, fmt.Errorf("no username found for: %s", username)
	}

	resp := response.Hits.Hits[0]

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
	return es.getRawRolePermissionEs7(ctx, role)
}
