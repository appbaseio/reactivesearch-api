package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

// FAQBody will contain the details about the FAQ
type FAQBody struct {
	Question    *string   `json:"question,omitempty"`
	Answer      *string   `json:"answer,omitempty"`
	SearchboxId *[]string `json:"searchboxId,omitempty"`
	ID          *string   `json:"faq_id,omitempty"`
	Order       *int      `json:"order,omitempty"`
	UpdatedAt   *int64    `json:"updated_at"`
}

// ValidateFAQBody will validate the FAQ body passed
// and make sure that it contains all the required values.
func ValidateFAQBody(bodyPassed FAQBody) error {
	if bodyPassed.Question == nil {
		return errors.New("`question` is a required field")
	}

	if bodyPassed.Answer == nil {
		return errors.New("`answer` is a required field")
	}

	if bodyPassed.SearchboxId == nil {
		return errors.New("`searchboxId` is a required field")
	}

	return nil
}

// SyncFAQToZinc will sync the FAQ's from ES to Zinc
//
// When this function runs, all the FAQ's will be fetched from ES
// and stored in Zinc.
//
// This function will pull all the records from ElasticSearch, delete all
// the existing docs from Zinc and index them into Zinc again.
func (zinc *FAQZinc) SyncFAQToZinc(index string, zincIndex string) error {
	// Pull all the docs from ES
	response, err := util.GetClient7().Search().
		Index(index).
		Size(10000).
		Sort("updated_at", false).
		Do(context.Background())

	if err != nil {
		errMsg := fmt.Sprint("error while pulling records from ES to save in Zinc: ", err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}

	// Iterate through the requests and build a bulk request body
	// to index the data into Zinc.
	zincBulkBody := make([]string, 0)
	zincIndexDoc := fmt.Sprintf(`{ "index" : { "_index" : "%s" } }`, zincIndex)

	for _, doc := range response.Hits.Hits {
		zincBulkBody = append(zincBulkBody, zincIndexDoc)
		zincBulkBody = append(zincBulkBody, strings.Replace(string(doc.Source), "\n", " ", -1))
	}

	// Make the x-ndjson request
	requestBody := strings.Join(zincBulkBody, "\n")

	// Add ending newline since nd-json should end with a new line.
	requestBody += "\n"

	// Delete the index if it already exists
	indexDeleteURL := fmt.Sprintf("/api/index/%s", zincIndex)
	reqHeaders := make(http.Header)
	reqHeaders.Add("Content-Type", "application/x-ndjson")

	deleteResponse, deleteErr := zinc.zincClient.MakeRequest(indexDeleteURL, http.MethodDelete, nil, &reqHeaders)
	if deleteErr != nil {
		errMsg := fmt.Sprint("error while deleting zinc index before indexing new data: ", deleteErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}

	// For delete, we can accept the following response codes:
	// 200: deleted
	// 400: something went wrong from our end, possibly index doesn't exist
	if deleteResponse.StatusCode != http.StatusOK && deleteResponse.StatusCode != http.StatusBadRequest {
		body, readErr := ioutil.ReadAll(deleteResponse.Body)
		if readErr == nil {
			log.Warnln(logTag, ": response received: ", string(body))
			log.Warnln(logTag, ": status received: ", deleteResponse.Status)
		}
		errMsg := fmt.Sprint("non OK status code received while deleting zinc index")
		log.Warnln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}

	// Create the index before making the bulk request

	indexCreateBody := fmt.Sprintf(zincMapping, zincIndex)

	// Send a create request for the index
	// with the mapping and name of the index present in the body
	indexCreateResponse, indexCreateErr := zinc.zincClient.MakeRequest("/api/index", http.MethodPost, []byte(indexCreateBody), nil)

	if indexCreateErr != nil {
		return fmt.Errorf("error while creating index named: %s, %v", zincIndex, indexCreateErr)
	}

	// Check status code and handle errors accordingly, if any
	if indexCreateResponse.StatusCode != http.StatusOK {
		useBody := false
		body, readErr := ioutil.ReadAll(indexCreateResponse.Body)
		if readErr == nil {
			useBody = true
		}
		errMsg := fmt.Sprintf("non OK status code received while creating index named `%s` with status code: %d", zincIndex, indexCreateResponse.StatusCode)
		if useBody {
			errMsg += fmt.Sprintf(" and message: %s", string(body))
		}
		return fmt.Errorf(errMsg)
	}

	bulkURL := fmt.Sprintf("/api/_bulk")
	bulkResponse, bulkErr := zinc.zincClient.MakeRequest(bulkURL, http.MethodPost, []byte(requestBody), nil)

	if bulkErr != nil {
		errMsg := fmt.Sprint("error while sending bulk request to zinc to index FAQ suggestions: ", bulkErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}

	if bulkResponse.StatusCode != http.StatusOK {
		body, readErr := ioutil.ReadAll(bulkResponse.Body)
		if readErr == nil {
			log.Warnln(logTag, ": response received: ", string(body))
			log.Warnln(logTag, ": status received: ", bulkResponse.Status)
		}
		return fmt.Errorf("non OK status code received while bulk creating FAQ's: %d", bulkResponse.StatusCode)
	}

	return nil
}

// getFAQNextOrder will get the next order value for
// FAQ by hitting ES.
func (r *OpenAI) getFAQNextOrder(ctx context.Context) int {
	// Get the next order by getting search hits with order as
	// descending.
	nextOrder, totalCount, nextOrderFetchErr := r.faqEs.getNextFAQOrder(ctx)

	// If error is nil, we can return the nextOrder value
	if nextOrderFetchErr == nil {
		return nextOrder
	}

	log.Warnln(logTag, ": error while fetching next FAQ order: ", nextOrderFetchErr.Error())
	if totalCount != 0 {
		return int(totalCount) + 1
	}

	// Use the fallback of getting total count from ES
	totalFAQCount, totalCountFetchErr := r.faqEs.getFAQCount(ctx)
	if totalCountFetchErr != nil {
		return 1
	}

	return int(totalFAQCount) + 1
}

// UpdatePartialFields will update the fields passed in the request
// body and return the updated FAQBody.
//
// The FAQBody will be fetched by using the passed ID.
func (r *OpenAI) UpdatePartialFields(ctx context.Context, faqBody FAQBody, faqId string) (FAQBody, error) {
	// Use the passed faqId to fetch the FAQ.
	faqBodyInBytes, fetchErr := r.faqEs.getFAQ(ctx, faqId)
	if fetchErr != nil {
		errMsg := fmt.Sprint("error while fetching FAQ with passed ID: ", fetchErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return FAQBody{}, fmt.Errorf(errMsg)
	}

	// Unmarshal the FAQ bytes into the structure.
	var faqBodyFromId FAQBody
	unmarshalErr := json.Unmarshal(faqBodyInBytes, &faqBodyFromId)
	if unmarshalErr != nil {
		errMsg := fmt.Sprint("error while unmarshaling faq body from bytes into structure: ", unmarshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return FAQBody{}, fmt.Errorf(errMsg)
	}

	if faqBody.Answer != nil {
		faqBodyFromId.Answer = faqBody.Answer
	}

	if faqBody.Question != nil {
		faqBodyFromId.Question = faqBody.Question
	}

	if faqBody.SearchboxId != nil {
		faqBodyFromId.SearchboxId = faqBody.SearchboxId
	}

	if faqBody.Order != nil {
		faqBodyFromId.Order = faqBody.Order
	}

	return faqBodyFromId, nil
}
