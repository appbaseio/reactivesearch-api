package users

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) getRawUsersEs7(ctx context.Context) ([]byte, error) {
	searchRequest := util.GetInternalClient7().Search().
		Index(es.indexName).
		Size(1000)

	response, err := util.SearchRequestDo(searchRequest, nil, ctx)

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
	// Fetch the userID
	userID, idFetchErr := es.getUserID(ctx, username)
	if idFetchErr != nil {
		return nil, idFetchErr
	}

	updateRequest := util.GetInternalClient7().Update().
		Refresh("wait_for").
		Index(es.indexName).
		Id(userID).
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
	// NOTE: Adding `tenant_id` to a get doc request is not possible
	// but we want to be able to filter based on tenant_id and also
	// remove the field accordingly so getting the user through search
	// is a better idea.
	usernameTermQuery := es7.NewTermQuery("username", username)

	searchRequest := util.GetInternalClient7().Search().Index(es.indexName).Query(usernameTermQuery).FetchSource(true).Size(1)

	response, err := util.SearchRequestDo(searchRequest, usernameTermQuery, ctx)

	if err != nil {
		return nil, err
	}

	// Use the first result from the batch since only 1 match will be found
	// based on the ID.

	// Add a check to throw an error if length is empty which would mean the
	// user is not present.
	if len(response.Hits.Hits) < 1 {
		return nil, fmt.Errorf("no user found with username `%s`", username)
	}

	responseToUse := response.Hits.Hits[0]

	src, err := responseToUse.Source.MarshalJSON()
	if err != nil {
		return nil, err
	}

	return src, nil
}

// getUserID will fetch the ID of the document for the username passed
func (es *elasticsearch) getUserID(ctx context.Context, username string) (string, error) {
	// NOTE: Adding `tenant_id` to a get doc request is not possible
	// but we want to be able to filter based on tenant_id and also
	// remove the field accordingly so getting the user through search
	// is a better idea.
	usernameTermQuery := es7.NewTermQuery("username", username)

	searchRequest := util.GetInternalClient7().Search().Index(es.indexName).Query(usernameTermQuery).FetchSource(true).Size(1)

	response, err := util.SearchRequestDo(searchRequest, usernameTermQuery, ctx)

	if err != nil {
		return "", err
	}

	// Use the first result from the batch since only 1 match will be found
	// based on the ID.

	// Add a check to throw an error if length is empty which would mean the
	// user is not present.
	if len(response.Hits.Hits) < 1 {
		return "", fmt.Errorf("no user found with username `%s`", username)
	}

	responseToUse := response.Hits.Hits[0]

	return responseToUse.Id, nil
}

func (es *elasticsearch) deleteUserEs7(ctx context.Context, username string) (bool, error) {
	// Fetch the userID
	userID, idFetchErr := es.getUserID(ctx, username)
	if idFetchErr != nil {
		return false, idFetchErr
	}

	deleteRequest := util.GetInternalClient7().Delete().
		Index(es.indexName).
		Refresh("wait_for").
		Id(userID)

	_, err := util.DeleteRequestDo(deleteRequest, ctx, username, es.indexName)
	if err != nil {
		return false, err
	}

	return true, nil
}
