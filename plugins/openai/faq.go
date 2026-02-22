package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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
