package openai

import (
	"context"
	"fmt"
	"log"

	"github.com/appbaseio/reactivesearch-api/util"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(openAIIndex, mapping string, actualMapping string) (*elasticsearch, error) {
	es := &elasticsearch{openAIIndex}

	ctx := context.Background()

	// Check if the openai index already exists
	exists, err := util.GetClient7().IndexExists(openAIIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, openAIIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas, actualMapping))

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(openAIIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", openAIIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, openAIIndex)
	return es, nil
}

type analyticsElasticsearch struct {
	indexName string
}

// initAnalyticsPlugin will initiate the analytics index for AI
func initAnalyticsPlugin(openAIAnalyticsIndex, mapping string) (*analyticsElasticsearch, error) {
	es := &analyticsElasticsearch{openAIAnalyticsIndex}

	ctx := context.Background()

	// Check if the rules index already exists
	exists, err := util.GetClient7().IndexExists(openAIAnalyticsIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, openAIAnalyticsIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas))

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(openAIAnalyticsIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", openAIAnalyticsIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, openAIAnalyticsIndex)
	return es, nil
}

func (es *elasticsearch) getSettings(ctx context.Context) (OpenAIConfig, error) {
	return es.getSettingsEs7(ctx)
}

// Following functions will be related to Analytics, all other functions
// should be defined above.

// saveSession will save the session based on the passed details
func (es *analyticsElasticsearch) saveSession(ctx context.Context, sessionDetails AISessionDoc, sessionId string) error {
	return es.saveSessionEs7(ctx, sessionDetails, sessionId)
}

// getSession will get the session details based on the passed sessionId
func (es *analyticsElasticsearch) getSession(ctx context.Context, sessionId string) (*AISessionDoc, error) {
	return es.getSessionEs7(ctx, sessionId)
}

// getSessionAnalytics will return the session analytics
func (es *analyticsElasticsearch) getAISessionAnalytics(ctx context.Context, from, to int64, size int) ([]byte, error) {
	return es.getAISessionAnalyticsEs7(ctx, from, to, size)
}

// filterSessionAnalytics will filter the responses based on the filters passed by the user
func (es *analyticsElasticsearch) filterAISessionAnalytics(ctx context.Context, queryParams FilterQueryParams) ([]AISessionDoc, error) {
	return es.filterAISessionAnalyticsEs7(ctx, queryParams)
}

type FAQElasticsearch struct {
	indexName string
}

// initFAQPluginES will initiate the analytics index for AI
func initFAQPluginES(faqIndex, mapping string) (*FAQElasticsearch, error) {
	es := &FAQElasticsearch{faqIndex}

	ctx := context.Background()

	// Check if the rules index already exists
	exists, err := util.GetClient7().IndexExists(faqIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, faqIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas))

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(faqIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", faqIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, faqIndex)
	return es, nil
}

// createFAQ will create/update the passed FAQ.
//
// The FAQ body should always contain the ID and is not
// checked in this function.
func (es *FAQElasticsearch) createFAQ(ctx context.Context, item FAQBody) error {
	return es.createFAQEs7(ctx, item)
}

// getFAQ will get the FAQ based on the passed ID
func (es *FAQElasticsearch) getFAQ(ctx context.Context, faqId string) ([]byte, error) {
	return es.getFAQEs7(ctx, faqId)
}

// deleteFAQ will delete the FAQ based on the passed ID
func (es *FAQElasticsearch) deleteFAQ(ctx context.Context, faqId string) error {
	return es.deleteFAQEs7(ctx, faqId)
}

// getFAQs will get the FAQ's from ES based on the passed params
func (es *FAQElasticsearch) getFAQs(ctx context.Context, from, size int) ([]byte, error) {
	return es.getFAQsEs7(ctx, from, size)
}

// getFAQsBySearchBox will get the FAQ's from ES based on the passed params
func (es *FAQElasticsearch) getFAQsBySearchBox(ctx context.Context, searchboxId string, from, size int) ([]byte, error) {
	return es.getFAQsBySearchBoxEs7(ctx, searchboxId, from, size)
}

// getFAQCount will return the FAQ count by getting it from ES
func (es *FAQElasticsearch) getFAQCount(ctx context.Context) (int64, error) {
	return es.getFAQCountEs7(ctx)
}

// getNextFAQOrder will return the next order count for FAQ's
func (es *FAQElasticsearch) getNextFAQOrder(ctx context.Context) (int, int64, error) {
	return es.getNextFAQOrderEs7(ctx)
}
