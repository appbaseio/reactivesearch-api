package users

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/util"
)

func (es *elasticsearch) getRawUsersEs7(ctx context.Context) ([]byte, error) {
	searchRequest := util.GetClient7().Search().
		Index(es.indexName).
		Size(1000)

	response, err := util.SearchRequestDo(searchRequest, ctx)

	if err != nil {
		return nil, err
	}

	var users []json.RawMessage
	for _, hit := range response.Hits.Hits {
		users = append(users, hit.Source)
	}

	return json.Marshal(users)
}

func (es *elasticsearch) patchUserEs7(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error) {
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

func (es *elasticsearch) getRawUserEs7(ctx context.Context, username string) ([]byte, error) {
	response, err := util.GetClient7().Get().
		Index(es.indexName).
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

func (es *elasticsearch) deleteUserEs7(ctx context.Context, username string) (bool, error) {
	deleteRequest := util.GetClient7().Delete().
		Index(es.indexName).
		Refresh("wait_for").
		Id(username)

	_, err := util.DeleteRequestDo(deleteRequest, ctx, username, es.indexName)
	if err != nil {
		return false, err
	}

	return true, nil
}
