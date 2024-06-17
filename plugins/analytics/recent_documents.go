package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// RecentDocument will contain details about the recent
// document viewed by an user.
type RecentDocument struct {
	DocumentId *string                 `json:"document_id"`
	Source     *map[string]interface{} `json:"source"`
	Users      *map[string]int64       `json:"users"`
	Index      *string                 `json:"index"`
}

// RecentDocumentPublic will be the visible recent document that
// the API will accept and return
type RecentDocumentPublic struct {
	DocumentId   *string                 `json:"document_id"`
	UserId       *string                 `json:"user_id"`
	Index        *string                 `json:"index"`
	Source       *map[string]interface{} `json:"source"`
	LastAccessed *int64                  `json:"last_accessed,omitempty"`
}

// ToInternal will convert the public document to internal
// document.
//
// This function will not validate the values to make sure that
// they are present.
//
// This function will only be used if the document doesn't
// already exist in the index. If it already exists, we will
// do an `_update_by_query` with a script to inject the
// `user_id` in the `users` map.
func (r *RecentDocumentPublic) ToInternal() RecentDocument {
	usersMap := map[string]int64{
		*r.UserId: time.Now().Unix(),
	}

	return RecentDocument{
		DocumentId: r.DocumentId,
		Users:      &usersMap,
		Index:      r.Index,
		Source:     r.Source,
	}
}

// ToPublic will convert the internal values into a public structure
func (r *RecentDocument) ToPublic(userId string) RecentDocumentPublic {
	// Extract the last_accessed time from the users map
	// TODO: Verify that last_accessed works properly since its a map of int64
	lastAccessed := (*r.Users)[userId]

	return RecentDocumentPublic{
		DocumentId:   r.DocumentId,
		Source:       r.Source,
		Index:        r.Index,
		UserId:       &userId,
		LastAccessed: &lastAccessed,
	}
}

// validateDocument will validate the incoming document and throw errors
// if there are any.
func validateDocument(document RecentDocumentPublic) error {
	if document.DocumentId == nil || *document.DocumentId == "" {
		return fmt.Errorf("`document_id` is a required value, cannot be empty")
	}

	if document.UserId == nil || *document.UserId == "" {
		return fmt.Errorf("`userId` is a required value, cannot be empty")
	}

	return nil
}

// GenerateRecentDocId will generate the recent document ID with the
// passed values.
func GenerateRecentDocId(documentId, index string) string {
	return fmt.Sprintf("%s__%s", documentId, index)
}

// FetchDocumentForSource will fetch the document based on the passed
// details
func FetchDocumentForSource(index string, documentId string) (map[string]interface{}, error) {
	errTemplate := fmt.Sprintf("Error while fetching doc with ID `%s` from index `%s`", documentId, index)

	requestOptions := es7.PerformRequestOptions{
		Method: "GET",
		Path:   fmt.Sprintf("/%s/_doc/%s", index, documentId),
	}
	response, docFetchErr := util.GetClient7().PerformRequest(context.Background(), requestOptions)
	if docFetchErr != nil {
		errMsg := fmt.Sprint(errTemplate, " with error: ", docFetchErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Parse the response and extract the `_source`
	if response.StatusCode != http.StatusOK {
		errMsg := fmt.Sprint(errTemplate, " : non OK status code received: ", response.StatusCode)
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Unmarshal the body into map
	responseAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(response.Body, &responseAsMap)
	if unmarshalErr != nil {
		errMsg := fmt.Sprint(errTemplate, " : error while unmarshaling ", unmarshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	source, isSourcePresent := responseAsMap["_source"]
	if !isSourcePresent {
		errMsg := fmt.Sprint(errTemplate, " : `_source` is not present")
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	sourceAsMap, asMapOk := source.(map[string]interface{})
	if !asMapOk {
		errMsg := fmt.Sprint(errTemplate, " : `_source` is not a map")
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	return sourceAsMap, nil
}
