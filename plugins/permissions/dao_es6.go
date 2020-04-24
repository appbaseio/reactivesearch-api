package permissions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/arc/util"
	es6 "gopkg.in/olivere/elastic.v6"
)

func (es *elasticsearch) checkRoleExistsEs6(ctx context.Context, role string) (bool, error) {
	resp, err := util.GetClient6().Search().
		Index(es.indexName).
		Query(es6.NewTermQuery("role", role)).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return resp.Hits.TotalHits > 0, nil
}

func (es *elasticsearch) getRawRolePermissionEs6(ctx context.Context, role string) ([]byte, error) {
	resp, err := util.GetClient6().Search().
		Index(es.indexName).
		Query(es6.NewTermQuery("role", role)).
		Size(1).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	for _, hit := range resp.Hits.Hits {
		src, err := json.Marshal(*hit.Source)
		if err == nil {
			return src, nil
		}
	}
	return nil, nil
}

func (es *elasticsearch) getRawOwnerPermissionsEs6(ctx context.Context, owner string) ([]byte, error) {
	resp, err := util.GetClient6().Search().
		Index(es.indexName).
		Query(es6.NewTermQuery("owner.keyword", owner)).
		Size(1000).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	rawPermissions := []json.RawMessage{}
	for _, hit := range resp.Hits.Hits {
		rawPermission, err := applyExpiredField(*hit.Source)
		if err != nil {
			return nil, err
		}
		rawPermissions = append(rawPermissions, rawPermission)
	}

	raw, err := json.Marshal(rawPermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal slice of raw permissions: %v", err)
	}

	return raw, nil
}

func (es *elasticsearch) getPermissionsEs6(ctx context.Context, indices []string) ([]byte, error) {
	query := es6.NewBoolQuery()
	util.GetIndexFilterQueryEs6(query, indices...)
	resp, err := util.GetClient6().Search().
		Index(es.indexName).
		Query(query).
		Size(1000).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	rawPermissions := []json.RawMessage{}
	if resp.Hits.TotalHits == 0 {
		return nil, fmt.Errorf("No permissions were found for index(es): %v", indices)
	}
	for _, hit := range resp.Hits.Hits {
		rawPermission, err := applyExpiredField(*hit.Source)
		if err != nil {
			return nil, err
		}
		rawPermissions = append(rawPermissions, rawPermission)
	}

	raw, err := json.Marshal(rawPermissions)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal slice of raw permissions: %v", err)
	}

	return raw, nil
}

func (es *elasticsearch) getRawPermissionEs6(ctx context.Context, username string) ([]byte, error) {
	response, err := util.GetClient6().Get().
		Index(es.indexName).
		Type(typeName).
		Id(username).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}
	src, err := applyExpiredField(*response.Source)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func (es *elasticsearch) patchPermissionEs6(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error) {
	response, err := util.GetClient6().Update().
		Refresh("wait_for").
		Index(es.indexName).
		Type(typeName).
		Id(username).
		Doc(patch).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	src, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return src, nil
}
