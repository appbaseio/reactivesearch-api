package permissions

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/util"
)

type elasticsearch struct {
	indexName string
	mapping   string
}

func initPlugin(indexName, mapping string) (*elasticsearch, error) {
	ctx := context.Background()

	es := &elasticsearch{indexName, mapping}

	// Check if the meta index already exists
	exists, err := util.GetClient7().IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while checking if index already exists: %v", logTag, err)
	}
	if exists {
		log.Println(logTag, ": index named", indexName, "already exists, skipping...")
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas)

	// Create a new meta index
	_, err = util.GetClient7().CreateIndex(indexName).
		Body(settings).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: error while creating index named %s: %v", logTag, indexName, err)
	}

	log.Println(logTag, ": successfully created index named", indexName)
	return es, nil
}

func applyExpiredField(data []byte) ([]byte, error) {
	var rawPermission *permission.Permission
	err := json.Unmarshal(data, &rawPermission)
	if err != nil {
		return nil, fmt.Errorf("unable to un-marshal slice of raw permissions: %v", err)
	}
	rawPermission.Expired, err = rawPermission.IsExpired()
	if err != nil {
		return nil, err
	}
	marshalled, err := json.Marshal(rawPermission)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal slice of raw permissions: %v", err)
	}
	return marshalled, nil
}

func (es *elasticsearch) getPermission(ctx context.Context, username string) (*permission.Permission, error) {
	raw, err := es.getRawPermission(ctx, username)
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

func (es *elasticsearch) getRawPermission(ctx context.Context, username string) ([]byte, error) {
	switch util.GetVersion() {
	case 6:
		return es.getRawPermissionEs6(ctx, username)
	default:
		return es.getRawPermissionEs7(ctx, username)
	}
}

func (es *elasticsearch) postPermission(ctx context.Context, p permission.Permission) (bool, error) {
	_, err := util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexName).
		Id(p.Username).
		BodyJson(p).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) patchPermission(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error) {
	switch util.GetVersion() {
	case 6:
		return es.patchPermissionEs6(ctx, username, patch)
	default:
		return es.patchPermissionEs7(ctx, username, patch)
	}
}

func (es *elasticsearch) deletePermission(ctx context.Context, username string) (bool, error) {
	_, err := util.GetClient7().Delete().
		Refresh("wait_for").
		Index(es.indexName).
		Id(username).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (es *elasticsearch) getRawOwnerPermissions(ctx context.Context, owner string) ([]byte, error) {
	switch util.GetVersion() {
	case 6:
		return es.getRawOwnerPermissionsEs6(ctx, owner)
	default:
		return es.getRawOwnerPermissionsEs7(ctx, owner)
	}
}

func (es *elasticsearch) getPermissions(ctx context.Context, indices []string) ([]byte, error) {
	switch util.GetVersion() {
	case 6:
		return es.getPermissionsEs6(ctx, indices)
	default:
		return es.getPermissionsEs7(ctx, indices)
	}
}

func (es *elasticsearch) checkRoleExists(ctx context.Context, role string) (bool, error) {
	switch util.GetVersion() {
	case 6:
		return es.checkRoleExistsEs6(ctx, role)
	default:
		return es.checkRoleExistsEs7(ctx, role)
	}
}

func (es *elasticsearch) getRawRolePermission(ctx context.Context, role string) ([]byte, error) {
	switch util.GetVersion() {
	case 6:
		return es.getRawRolePermissionEs6(ctx, role)
	default:
		return es.getRawRolePermissionEs7(ctx, role)
	}
}
