package permissions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) checkRoleExistsEs7(ctx context.Context, role string) (bool, error) {
	searchRequest := util.GetClient7().Search().
		Index(es.indexName).
		Query(es7.NewTermQuery("role", role))

	resp, err := util.SearchRequestDo(searchRequest, ctx)
	if err != nil {
		return false, err
	}

	return resp.Hits.TotalHits.Value > 0, nil
}

func (es *elasticsearch) getRawRolePermissionEs7(ctx context.Context, role string) ([]byte, error) {
	searchRequest := util.GetClient7().Search().
		Index(es.indexName).
		Query(es7.NewTermQuery("role", role)).
		Size(1).
		FetchSource(true)

	resp, err := util.SearchRequestDo(searchRequest, ctx)
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

func (es *elasticsearch) getRawOwnerPermissionsEs7(ctx context.Context, owner string) ([]byte, error) {
	searchRequest := util.GetClient7().Search().
		Index(es.indexName).
		Query(es7.NewTermQuery("owner.keyword", owner)).
		Size(10000)

	resp, err := util.SearchRequestDo(searchRequest, ctx)
	if err != nil {
		return nil, err
	}

	rawPermissions := []json.RawMessage{}
	for _, hit := range resp.Hits.Hits {
		rawPermission, err := applyExpiredField(hit.Source)
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

func (es *elasticsearch) getPermissionsEs7(ctx context.Context, indices []string) ([]byte, error) {
	query := es7.NewBoolQuery()
	util.GetIndexFilterQueryEs7(query, indices...)
	searchRequest := util.GetClient7().Search().
		Index(es.indexName).
		Query(query).
		Size(10000)

	resp, err := util.SearchRequestDo(searchRequest, ctx)

	if err != nil {
		return nil, err
	}

	rawPermissions := []json.RawMessage{}
	for _, hit := range resp.Hits.Hits {
		rawPermission, err := applyExpiredField(hit.Source)
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

func (es *elasticsearch) getRawPermissionEs7(ctx context.Context, username string) ([]byte, error) {
	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id(username).
		FetchSource(true).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	src, err := applyExpiredField(response.Source)
	if err != nil {
		return nil, err
	}
	return src, nil
}

func (es *elasticsearch) patchPermissionEs7(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error) {
	updateRequest := util.GetClient7().Update().
		Refresh("wait_for").
		Index(es.indexName).
		Id(username).
		Doc(patch)

	response, err := util.UpdateRequestDo(updateRequest, patch, ctx)
	if err != nil {
		return nil, err
	}

	src, err := json.Marshal(response)
	if err != nil {
		return nil, err
	}

	return src, nil
}
