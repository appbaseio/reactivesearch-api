package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"

	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/buger/jsonparser"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// getSettings will return the OpenAI settings
func (es *elasticsearch) getSettingsEs7(ctx context.Context) (OpenAIConfig, error) {
	var record = OpenAIConfig{}
	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id(openAIConfigDocID).
		Do(ctx)
	if err != nil {
		log.Warnln(logTag, ": preferences not found", err)
		return GetDefaultConfig(), nil
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error retrieving cache preferences", err)
		return GetDefaultConfig(), err
	}
	return record, nil
}

// saveSettings will save the passed OpenAI settings
func (es *elasticsearch) saveSettings(openAISettings OpenAIConfig, ctx context.Context) error {
	_, err := util.GetClient7().
		Update().
		Index(es.indexName).
		Upsert(openAISettings).
		DocAsUpsert(true).
		Doc(openAISettings).
		RetryOnConflict(5).
		Id(openAIConfigDocID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating openAI config :", err)
		return err
	}
	return nil
}

// Following functions will be related to Analytics, all other functions
// should be defined above.

// saveSessionEs7 will save the session details passed
func (es *analyticsElasticsearch) saveSessionEs7(ctx context.Context, sessionDetails AISessionDoc, sessionId string) error {
	_, err := util.GetClient7().
		Update().
		Index(es.indexName).
		Upsert(sessionDetails).
		DocAsUpsert(true).
		Id(sessionId).
		Doc(sessionDetails).Do(ctx)

	if err != nil {
		log.Warnln(logTag, ": error while inserting/updating session doc with err: ", err.Error())
		return err
	}

	return nil
}

// getSessionEs7 will get the saved session based on the passed sessionId
func (es *analyticsElasticsearch) getSessionEs7(ctx context.Context, sessionId string) (*AISessionDoc, error) {
	response, err := util.GetClient7().
		Get().
		Id(sessionId).
		Index(es.indexName).
		Do(ctx)

	if err != nil {
		log.Warnln(logTag, ": error while fetching doc for sessionId: ", err.Error())
		return nil, err
	}

	var responseAsType AISessionDoc

	unmarshalErr := json.Unmarshal(response.Source, &responseAsType)
	if unmarshalErr != nil {
		log.Warnln(logTag, ": error while unmarshalling session details into custom type: ", unmarshalErr.Error())
		return nil, unmarshalErr
	}

	return &responseAsType, nil
}

// getAISessionAnalytics will return the session analytics for
// AI sessions
func (es *analyticsElasticsearch) getAISessionAnalyticsEs7(ctx context.Context, from int64, to int64, size int) ([]byte, error) {
	timeRange := es7.NewRangeQuery("created_at").Gte(from).Lte(to)
	query := es7.NewBoolQuery().Filter(timeRange)

	// Add user term aggregation
	userAggr := es7.NewTermsAggregation().Field("user_id.keyword").Size(size).OrderByCountDesc()
	invocationAggr := es7.NewTermsAggregation().Field("invocation_count").Size(size).OrderByCountDesc()
	inputTokenCountAggr := es7.NewSumAggregation().Field("input_tokens")
	outputTokenCountAggr := es7.NewSumAggregation().Field("output_tokens")
	usefulAggr := es7.NewTermsAggregation().Field("useful").Size(size)
	uniqueUserCountAggr := es7.NewCardinalityAggregation().Field("user_id.keyword")
	modelAggr := es7.NewTermsAggregation().Field("model.keyword").Size(size)
	totalInvocationAggr := es7.NewSumAggregation().Field("invocation_count")

	// Add the date based aggregation with a sub aggregation for total_invocations
	totalInvocationSubAggr := es7.NewSumAggregation().Field("invocation_count")
	timestampAggr := es7.NewDateHistogramAggregation().Field("timestamp").FixedInterval("1d").
		SubAggregation("total_invocations", totalInvocationSubAggr)

	result, esErr := util.GetClient7().Search(es.indexName).Query(query).Size(0).
		TrackTotalHits(true).
		Aggregation("users", userAggr).
		Aggregation("invocations", invocationAggr).
		Aggregation("total_input_tokens", inputTokenCountAggr).
		Aggregation("total_output_tokens", outputTokenCountAggr).
		Aggregation("useful", usefulAggr).
		Aggregation("user_count", uniqueUserCountAggr).
		Aggregation("models", modelAggr).
		Aggregation("total_invocations", totalInvocationAggr).
		Aggregation("session_histogram", timestampAggr).
		Do(ctx)

	if esErr != nil {
		return nil, fmt.Errorf("error while fetching aggregation results from ES: %s", esErr.Error())
	}

	resultMap := make(map[string]interface{})
	defaultEmptyBucket := []map[string]interface{}{}

	// Extract users bucket
	userBucket, userBucketErr := extractBucketsFromResponse(result, "users")
	if userBucketErr != nil {
		return nil, userBucketErr
	}

	if userBucket == nil {
		userBucket = defaultEmptyBucket
	}
	resultMap["users"] = userBucket

	// Extract invocations
	invocationBucket, invocationBucketErr := extractBucketsFromResponse(result, "invocations")
	if invocationBucketErr != nil {
		return nil, invocationBucketErr
	}

	if invocationBucket == nil {
		invocationBucket = defaultEmptyBucket
	}
	resultMap["invocations"] = invocationBucket

	// Extract total_input_tokens
	totalInputTokens, tokenErr := extractSumFromResponse(result, "total_input_tokens")
	if tokenErr != nil {
		return nil, tokenErr
	}
	resultMap["total_input_tokens"] = totalInputTokens

	// Extract total_output_tokens
	totalOutputTokens, tokenErr := extractSumFromResponse(result, "total_output_tokens")
	if tokenErr != nil {
		return nil, tokenErr
	}
	resultMap["total_output_tokens"] = totalOutputTokens

	// Calculate the total tokens as well
	resultMap["total_tokens"] = totalInputTokens + totalOutputTokens

	// Extract useful buckets
	usefulBucket, usefulBucketErr := extractBucketsFromResponse(result, "useful")
	if usefulBucketErr != nil {
		return nil, usefulBucketErr
	}

	if usefulBucket == nil {
		usefulBucket = defaultEmptyBucket
	}
	resultMap["useful"] = usefulBucket

	// Extract the unique user count
	userCountResult, found := result.Aggregations.Cardinality("user_count")
	if !found {
		return nil, fmt.Errorf("no aggregation found for '%s'", "user_count")
	}
	resultMap["user_count"] = userCountResult.Value

	// Extract models
	modelsBucket, modelBucketErr := extractBucketsFromResponse(result, "models")
	if modelBucketErr != nil {
		return nil, modelBucketErr
	}

	if modelsBucket == nil {
		modelsBucket = defaultEmptyBucket
	}
	resultMap["models"] = modelsBucket

	// Extract the total invocation count
	totalInvocationCount, invocationErr := extractSumFromResponse(result, "total_invocations")
	if invocationErr != nil {
		return nil, invocationErr
	}
	resultMap["total_invocations"] = totalInvocationCount

	// Extract the date histogram response
	histogramResult, sessionHistogramParseErr := extractSessionHistogram(result, "session_histogram", "total_invocations")
	if sessionHistogramParseErr != nil {
		return nil, sessionHistogramParseErr
	}
	resultMap["session_histogram"] = histogramResult

	// Finally, inject the total hits as well
	resultMap["total_sessions"] = result.Hits.TotalHits.Value

	return json.Marshal(resultMap)
}

// filterAISessionAnalyticsEs7 will filter the analytics based on the passed values
// and accordingly return a response
func (es *analyticsElasticsearch) filterAISessionAnalyticsEs7(ctx context.Context, queryParams FilterQueryParams) ([]AISessionDoc, error) {
	boolQuery := es7.NewBoolQuery()

	// isFilterUsed will indicate whether or not the filter
	// is used so that accordingly a fallback query can be
	// built
	isFilterUsed := false

	// Invocation will have a range query
	if queryParams.InvocationMin != nil || queryParams.InvocationMax != nil {
		invocationRangeQuery := es7.NewRangeQuery("invocation_count")

		if queryParams.InvocationMin != nil {
			invocationRangeQuery = invocationRangeQuery.Gte(queryParams.InvocationMin)
		}

		if queryParams.InvocationMax != nil {
			invocationRangeQuery = invocationRangeQuery.Lte(queryParams.InvocationMax)
		}

		isFilterUsed = true
		boolQuery = boolQuery.Must(invocationRangeQuery)
	}

	// Timestamp will have a range query
	if queryParams.FromTimeStamp != nil || queryParams.ToTimeStamp != nil {
		timestampRangeQuery := es7.NewRangeQuery("timestamp")

		if queryParams.FromTimeStamp != nil {
			timestampRangeQuery = timestampRangeQuery.From(queryParams.FromTimeStamp)
		}

		if queryParams.ToTimeStamp != nil {
			timestampRangeQuery = timestampRangeQuery.To(queryParams.ToTimeStamp)
		}

		isFilterUsed = true
		boolQuery = boolQuery.Must(timestampRangeQuery)
	}

	if queryParams.Useful != nil {
		usefulTermsQuery := es7.NewTermQuery("useful", *queryParams.Useful)
		isFilterUsed = true
		boolQuery = boolQuery.Must(usefulTermsQuery)
	}

	if queryParams.ModelPrefix != nil && len(*queryParams.ModelPrefix) > 0 {
		// We will have to create a nested should query here
		nestedBoolQuery := es7.NewBoolQuery()

		for _, modelPrefix := range *queryParams.ModelPrefix {
			nestedBoolQuery = nestedBoolQuery.Should(es7.NewMatchQuery("model", modelPrefix))
		}

		isFilterUsed = true
		boolQuery = boolQuery.Must(nestedBoolQuery)
	}

	if queryParams.UserId != nil && len(*queryParams.UserId) > 0 {
		// We will have to use a terms query here

		// Convert the array into an interface array
		values := []interface{}{}
		for _, user := range *queryParams.UserId {
			values = append(values, user)
		}

		isFilterUsed = true
		userIdTermsQuery := es7.NewTermsQuery("user_id", values...)
		boolQuery = boolQuery.Must(userIdTermsQuery)
	}

	// Size should be present regardless, if not then set to default
	if queryParams.Size == nil {
		defaultSize := 30
		queryParams.Size = &defaultSize
	}

	if queryParams.Offset == nil {
		defaultOffset := 0
		queryParams.Offset = &defaultOffset
	}

	// Build the search service
	searchService := util.GetClient7().Search().Index(es.indexName)

	// Handle scenario where all filters are empty
	if !isFilterUsed {
		searchService = searchService.Query(es7.NewMatchAllQuery())
	} else {
		searchService = searchService.Query(boolQuery)
	}

	results, esErr := searchService.
		Size(*queryParams.Size).
		From(*queryParams.Offset).
		Do(ctx)

	if esErr != nil {
		errMsg := fmt.Errorf("error while fetching filter results from ES: %s", esErr.Error())
		return nil, errMsg
	}

	// Parse the response now
	sessionDocs := make([]AISessionDoc, 0)
	for _, hit := range results.Hits.Hits {
		var sessionDocEach AISessionDoc
		unmarshalErr := json.Unmarshal(hit.Source, &sessionDocEach)
		if unmarshalErr != nil {
			log.Warnln(logTag, ": error while unmarshalling source: ", unmarshalErr.Error())
			continue
		}

		// Remove the internal response
		sessionDocEach.InternalResponse = nil

		sessionDocs = append(sessionDocs, sessionDocEach)
	}

	return sessionDocs, nil
}

// Following functions will be related to FAQ, all other functions should
// be defined in their section.

// createFAQEs7 will create the FAQ in ElasticSearch
func (es *FAQElasticsearch) createFAQEs7(ctx context.Context, item FAQBody) error {
	// Extract the ID to use
	//
	// NOTE: We can safely assume that the ID will not be nil
	// since this will be checked in the parent.
	idToUse := item.ID

	_, err := util.GetClient7().Update().
		Index(es.indexName).
		DocAsUpsert(true).
		RetryOnConflict(5).
		Refresh("wait_for").
		Id(*idToUse).
		Doc(item).
		Do(ctx)

	if err != nil {
		errMsg := fmt.Sprint("something went wrong while inserting the faq into index: ", err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}

	return nil
}

// deleteFAQEs7 will delete the FAQ in ElasticSearch
func (es *FAQElasticsearch) deleteFAQEs7(ctx context.Context, faqId string) error {
	_, err := util.GetClient7().Delete().
		Index(es.indexName).
		Refresh("wait_for").
		Id(faqId).
		Do(ctx)

	if err != nil {
		errMsg := fmt.Sprint("error while deleting FAQ: ", err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return errors.New(errMsg)
	}

	return nil
}

// getFAQEs7 will get the FAQ by the passed ID and return it accordingly
func (es *FAQElasticsearch) getFAQEs7(ctx context.Context, faqId string) ([]byte, error) {
	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id(faqId).
		Do(ctx)

	if err != nil {
		errMsg := fmt.Sprint("Error while getting FAQ doc: ", err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return nil, errors.New(errMsg)
	}

	// Unmarshall source into FAQBody structure and return that marshalled
	var faqBody FAQBody
	unmarshalErr := json.Unmarshal(response.Source, &faqBody)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	if faqBody.Order == nil {
		defaultOrder := 1
		faqBody.Order = &defaultOrder
	}

	return json.Marshal(faqBody)
}

// getFAQsEs7 will get the FAQ's based on the passed query params
func (es *FAQElasticsearch) getFAQsEs7(ctx context.Context, from, size int) ([]byte, error) {
	response, err := util.GetClient7().Search().
		Index(es.indexName).
		From(from).
		Size(size).
		Sort("updated_at", false).
		Do(ctx)

	if err != nil {
		errMsg := fmt.Sprint("error while getting FAQ's: ", err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return nil, errors.New(errMsg)
	}

	defaultOrder := 1
	faqs := make([]FAQBody, 0)
	for index, hit := range response.Hits.Hits {
		var faqEach FAQBody
		unmarshalErr := json.Unmarshal(hit.Source, &faqEach)
		if unmarshalErr != nil {
			log.Warnln(logTag, ": error while unmarshalling hit at index: ", index, " with error: ", unmarshalErr.Error())
			continue
		}

		if faqEach.Order == nil {
			faqEach.Order = &defaultOrder
		}

		faqs = append(faqs, faqEach)
	}

	// We should sort the faq's on `order` field. This is because, the `order` field
	// may not be present in the mappings and sorting on it through ES might lead to
	// an error.
	//
	// Sort the faq's in ascending order of `order` value.
	sort.Slice(faqs, func(i, j int) bool {
		return *faqs[i].Order < *faqs[j].Order
	})

	return json.Marshal(faqs)
}

// getFAQsBySearchBoxEs7 will get the FAQ's for a searchbox based on the passed query params
func (es *FAQElasticsearch) getFAQsBySearchBoxEs7(ctx context.Context, searchboxId string, from, size int) ([]byte, error) {
	searchBoxFilter := es7.NewTermQuery("searchboxId.keyword", searchboxId)
	response, err := util.GetClient7().Search().Index(es.indexName).
		Query(searchBoxFilter).
		From(from).
		Size(size).
		Sort("updated_at", false).
		Do(ctx)

	if err != nil {
		errMsg := fmt.Sprint("error while getting FAQs by searchBoxId: ", err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return nil, errors.New(errMsg)
	}

	defaultOrder := 1
	faqs := make([]FAQBody, 0)
	for index, hit := range response.Hits.Hits {
		var faqEach FAQBody
		unmarshalErr := json.Unmarshal(hit.Source, &faqEach)
		if unmarshalErr != nil {
			log.Warnln(logTag, ": error while unmarshalling hit at index: ", index, " with error: ", unmarshalErr.Error())
			continue
		}

		if faqEach.Order == nil {
			faqEach.Order = &defaultOrder
		}

		faqs = append(faqs, faqEach)
	}

	// We should sort the faq's on `order` field. This is because, the `order` field
	// may not be present in the mappings and sorting on it through ES might lead to
	// an error.
	//
	// Sort the faq's in ascending order of `order` value.
	sort.Slice(faqs, func(i, j int) bool {
		return *faqs[i].Order < *faqs[j].Order
	})

	return json.Marshal(faqs)
}

// getFAQCountEs7 will get the total number of FAQ docs present
func (es *FAQElasticsearch) getFAQCountEs7(ctx context.Context) (int64, error) {
	faqCount, err := util.GetClient7().Count().
		Index(es.indexName).
		Do(ctx)

	if err != nil {
		errMsg := fmt.Sprint("error while fetching count of FAQ docs: ", err.Error())
		return 0, fmt.Errorf(errMsg)
	}

	return faqCount, nil
}

// getNextFAQOrderEs7 will get the order for the next FAQ doc by
// searching with a match_all and sorting on the order field.
func (es *FAQElasticsearch) getNextFAQOrderEs7(ctx context.Context) (int, int64, error) {
	response, err := util.GetClient7().Search().
		Index(es.indexName).
		Size(1).
		Sort("order", false).
		Do(ctx)

	if err != nil {
		return 0, 0, err
	}

	// If there are not hits, we can set the order as 1
	if len(response.Hits.Hits) == 0 {
		return 1, 0, nil
	}

	var faqEach FAQBody
	unmarshalErr := json.Unmarshal(response.Hits.Hits[0].Source, &faqEach)
	if unmarshalErr != nil {
		return 0, 0, unmarshalErr
	}

	if faqEach.Order == nil {
		return 0, response.Hits.TotalHits.Value, fmt.Errorf("error getting order for top hit.")
	}

	return *faqEach.Order + 1, response.Hits.TotalHits.Value, nil
}

// createFAQZinc will create a new FAQ item based on the passed
// details
func (zinc *FAQZinc) createFAQZinc(item FAQBody) error {
	// Extract the ID to use
	//
	// NOTE: We can safely assume that the ID will not be nil
	// since this will be checked in the parent.
	idToUse := item.ID

	// Marshal the item
	itemMarshalled, marshalErr := json.Marshal(item)
	if marshalErr != nil {
		return marshalErr
	}

	createURL := fmt.Sprintf("/es/%s/_doc/%s", zinc.indexName, *idToUse)
	createResponse, createErr := zinc.zincClient.MakeRequest(createURL, http.MethodPut, itemMarshalled, nil)

	if createErr != nil {
		return fmt.Errorf("error while creating document with error: %s", createErr.Error())
	}

	// The above endpoint will overwrite the doc if it
	// already exists, thus upsert-ing it.
	if createResponse.StatusCode != http.StatusOK {
		body, readErr := ioutil.ReadAll(createResponse.Body)
		if readErr == nil {
			log.Warnln(logTag, ": response received: ", string(body))
			log.Warnln(logTag, ": status received: ", createResponse.Status)
		}
		return fmt.Errorf("non OK status code received while creating FAQ: %d", createResponse.StatusCode)
	}

	return nil
}

// getFAQZinc will get the FAQ item by using the passed ID
func (zinc *FAQZinc) getFAQZinc(faqId string) ([]byte, error) {
	getURL := fmt.Sprintf("/api/%s/_doc/%s", zinc.indexName, faqId)
	getResponse, getErr := zinc.zincClient.MakeRequest(getURL, http.MethodGet, []byte(""), nil)
	if getErr != nil {
		return nil, fmt.Errorf("error while getting the document from Zinc: %s", getErr.Error())
	}

	// If the status code is 404, return that accordingly
	if getResponse.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("FAQ not found with the passed ID")
	}

	if getResponse.StatusCode != http.StatusOK {
		// Something went wrong
		body, readErr := ioutil.ReadAll(getResponse.Body)
		if readErr == nil {
			log.Warnln(logTag, ": response received: ", string(body))
			log.Warnln(logTag, ": status received: ", getResponse.Status)
		}
		return nil, fmt.Errorf("non OK status code received while getting FAQ: %d", getResponse.StatusCode)
	}

	// Since everything was good, we can extract the value
	bodyRead, readErr := ioutil.ReadAll(getResponse.Body)
	if readErr != nil {
		return nil, fmt.Errorf("error while reading body received from Zinc: %s", readErr.Error())
	}

	faqSource, _, _, faqSrcErr := jsonparser.Get(bodyRead, "_source")
	return faqSource, faqSrcErr
}

// deleteFAQZinc will delete the FAQ item by using the passed ID
func (zinc *FAQZinc) deleteFAQZinc(faqId string) error {
	deleteURL := fmt.Sprintf("/es/%s/_delete_by_query", zinc.indexName)
	deleteBody := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"faq_id": faqId,
			},
		},
	}

	bodyMarshalled, marshalErr := json.Marshal(deleteBody)
	if marshalErr != nil {
		return marshalErr
	}

	deleteResponse, deleteErr := zinc.zincClient.MakeRequest(deleteURL, http.MethodPost, bodyMarshalled, nil)
	if deleteErr != nil {
		return deleteErr
	}

	if deleteResponse.StatusCode != http.StatusOK {
		body, readErr := ioutil.ReadAll(deleteResponse.Body)
		if readErr == nil {
			log.Warnln(logTag, ": response received: ", string(body))
			log.Warnln(logTag, ": status received: ", deleteResponse.Status)
		}
		return fmt.Errorf("non OK status code received while deleting FAQ: %d", deleteResponse.StatusCode)
	}

	return nil
}

// getFAQsZinc will get the FAQ's from Zinc
func (zc *FAQZinc) getFAQsZinc(from, size int) ([]byte, error) {
	searchBody := map[string]interface{}{
		"sort": []interface{}{
			map[string]interface{}{
				"updated_at": "desc",
			},
		},
		"from": from,
		"size": size,
	}

	// Marshal the search body
	// NOTE: Since we created the body manually, no need to handle marshal
	// error
	marshalledBody, _ := json.Marshal(searchBody)

	searchURL := fmt.Sprintf("/es/%s/_search", zc.indexName)
	searchResponse, searchErr := zc.zincClient.MakeRequest(searchURL, http.MethodPost, marshalledBody, nil)

	if searchErr != nil {
		return nil, searchErr
	}

	// Read the search response body and return it
	searchResult, readErr := ioutil.ReadAll(searchResponse.Body)
	return searchResult, readErr
}
