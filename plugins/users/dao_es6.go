package users

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/arc/util"
)

func (es *elasticsearch) getRawUsersEs6(ctx context.Context) ([]byte, error) {
	response, err := util.GetClient6().Search().
		Index(es.indexName).
		Size(1000).
		Do(ctx)

	if err != nil {
		return nil, err
	}

	var users []json.RawMessage
	for _, hit := range response.Hits.Hits {
		users = append(users, *hit.Source)
	}

	return json.Marshal(users)
}

func (es *elasticsearch) patchUserEs6(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error) {
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

func (es *elasticsearch) getRawUserEs6(ctx context.Context, username string) ([]byte, error) {
	response, err := util.GetClient6().Get().
		Index(es.indexName).
		Type(typeName).
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

func (es *elasticsearch) deleteUserEs6(ctx context.Context, username string) (bool, error) {
	_, err := util.GetClient6().Delete().
		Index(es.indexName).
		Refresh("wait_for").
		Type(typeName).
		Id(username).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}
