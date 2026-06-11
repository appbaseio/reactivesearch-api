package pipelines

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/plugins/suggestions"
	"github.com/appbaseio/reactivesearch-api/plugins/uibuilder"
	"github.com/appbaseio/reactivesearch-api/util"

	"github.com/appbaseio/reactivesearch-api/plugins/cache"
	"github.com/buger/jsonparser"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type ElasticsearchQueryInput struct {
	Method                        *string            `json:"method,omitempty" jsonschema:"title=Request Method" jsonschema_description:"Http request method, for e.g, 'POST'. The default value is the pipeline request method."`
	Path                          *string            `json:"path,omitempty" jsonschema:"title=Path" jsonschema_description:"URL path, for e.g '/_search'."`
	URL                           *string            `json:"url,omitempty" jsonschema:"title=Request URL" jsonschema_description:"Elasticsearch URL, for e.g, 'https://@appbase-demo-ansible-abxiydt-arc.searchbase.io'. The URL can have path and query params too, for instance, 'https://appbase-demo-ansible-abxiydt-arc.searchbase.io/_cat/indices?format=JSON'."`
	Params                        *map[string]string `json:"params,omitempty" jsonschema:"title=Query params" jsonschema_description:"Request query params, for e.g, '{ format: 'JSON' }'"`
	Headers                       *map[string]string `json:"headers,omitempty" jsonschema:"title=Request Headers" jsonschema_description:"Request headers, for e.g, '{ Content-Type: 'application/json' }'"`
	Body                          *string            `json:"body,omitempty" jsonschema:"title=Body" jsonschema_description:"Request body in string format, e.g, {\"query\":{\"match_all\":{}}}."`
	IndependentBody               *string            `json:"independentBody,omitempty" jsonschema:"title=Independent Body" jsonschema_description:"Array of independent requests, i:e ones that contain the endpoint in the query."`
	ParseResponseToReactivesearch *bool              `json:"parseResponseToReactivesearch,omitempty" jsonschema:"title=Parse Response to ReactiveSearch API" jsonschema_description:"If set to 'true', then it would transform the Elasticsearch response to RS API response. Defaults to 'true', if route category is 'reactivesearch'."`
	SetResponseToKV               *string            `json:"setResponseToKV,omitempty" jsonschema:"title=Set Response To KV" jsonschema_description:"Sets the response of the stage output as a value in the KV store with the provided key value. Accepts dynamic inputs using the {{{ mustache }}} syntax. E.g. pass envs as {{{envs.query}}} for the key to be set as the query value."`
}

func GetElasticsearchQueryInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&ElasticsearchQueryInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// Returns the final envs to be used
// Merges the script context envs to user envs
// User envs have priority
func getInputs(
	scriptEnvs ElasticsearchQueryInput,
	stageInputs *string) (ElasticsearchQueryInput, error) {
	finalEnvs := scriptEnvs
	if stageInputs == nil {
		return finalEnvs, nil
	}

	var parsedESInputs ElasticsearchQueryInput
	err2 := json.Unmarshal([]byte(*stageInputs), &parsedESInputs)
	if err2 != nil {
		return finalEnvs, err2
	}
	// merge inputs
	if parsedESInputs.Body != nil {
		finalEnvs.Body = parsedESInputs.Body
	}

	if parsedESInputs.IndependentBody != nil {
		finalEnvs.IndependentBody = parsedESInputs.IndependentBody
	}

	if parsedESInputs.SetResponseToKV != nil {
		finalEnvs.SetResponseToKV = parsedESInputs.SetResponseToKV
	}

	if parsedESInputs.Path != nil {
		finalEnvs.Path = parsedESInputs.Path
	}
	if parsedESInputs.URL != nil && *parsedESInputs.URL != "" {
		esURL := *parsedESInputs.URL
		if strings.Contains(esURL, "@") {
			splitIndex := strings.LastIndex(esURL, "@")
			protocolWithCredentials := strings.Split(esURL[0:splitIndex], "://")
			if len(protocolWithCredentials) > 1 {
				credentials := protocolWithCredentials[1]
				protocol := protocolWithCredentials[0]
				host := esURL[splitIndex+1:]

				credentialSeparator := strings.Index(credentials, ":")
				username := credentials[0:credentialSeparator]
				password := credentials[credentialSeparator+1:]
				esURL = protocol + "://" + url.PathEscape(username) + ":" + url.PathEscape(password) + "@" + host
			}
		}
		finalEnvs.URL = &esURL
	}
	if parsedESInputs.Method != nil {
		finalEnvs.Method = parsedESInputs.Method
	}
	if parsedESInputs.ParseResponseToReactivesearch != nil {
		finalEnvs.ParseResponseToReactivesearch = parsedESInputs.ParseResponseToReactivesearch
	}
	if parsedESInputs.Headers != nil {
		finalEnvs.Headers = parsedESInputs.Headers
	}
	if parsedESInputs.Params != nil {
		finalEnvs.Params = parsedESInputs.Params
	}
	return finalEnvs, nil
}

func executeElasticsearchStage(
	stage ESPipelineStage,
	parsedInputs *string,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	id := getStageID(stage)
	scriptContextInBytes := globalScriptContext.Get()
	// apply headers from script context
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}
	// Forward the request to elasticsearch
	// Skip adding the Accept header since it is passed by default as */* and Elastic doesn't like that and ends up throwing
	// Invalid media-type value on header [Accept] [type=media_type_header_exception]
	headers := http.Header{}
	for k := range scriptContext.Request.Headers {
		if k == "Authorization" || k == "Accept" {
			continue
		}
		headers.Set(k, scriptContext.Request.Headers[k])
	}

	// disable gzip compression
	encoding := headers.Get("Accept-Encoding")
	if encoding != "" {
		headers.Set("Accept-Encoding", "identity")
	}

	var reqURL string
	// extract request method
	method := http.MethodGet
	// Extract path from envs
	path, ok := scriptEnvs["path"].(string)
	if ok {
		reqURL = path
	}
	indices := getIndicesFromEnvs(scriptEnvs)

	reqMethod, ok := scriptEnvs["method"].(string)
	if ok {
		method = reqMethod
	}

	paramsMap := make(map[string]string)
	paramsAsMap, ok := scriptEnvs["urlValues"].(map[string]interface{})
	if ok {
		for k, v := range paramsAsMap {
			// NOTE: `debug` is not a valid query param for ES, so
			// we will have to ignore it
			if k == "debug" {
				continue
			}

			paramValue, ok := v.(string)
			if ok {
				paramsMap[k] = paramValue
			}
		}
	}

	headersMap := make(map[string]string)
	for k := range headers {
		headersMap[k] = headers.Get(k)
	}
	transformToRSAPIResponse := false
	if reqCategory, ok := scriptContext.Environments["category"]; ok {
		categoryAsString, ok := reqCategory.(string)
		if ok {
			if categoryAsString == category.ReactiveSearch.String() {
				// read RS API from channel
				transformToRSAPIResponse = true
			}
		}
	}
	// default ES URL from envs
	elasticsearchURL := util.GetESURL()

	independentRequest := ""
	independentRequestFromCtx, isIndependentOk := scriptContext.Environments["INDEPENDENT_REQUESTS"]
	if isIndependentOk {
		independentRequest = independentRequestFromCtx.(string)
	}

	inputs, err := getInputs(ElasticsearchQueryInput{
		URL:                           &elasticsearchURL,
		Method:                        &method,
		Path:                          &reqURL,
		Body:                          &scriptContext.Request.Body,
		IndependentBody:               &independentRequest,
		Params:                        &paramsMap,
		Headers:                       &headersMap,
		ParseResponseToReactivesearch: &transformToRSAPIResponse,
	}, parsedInputs)
	if err != nil {
		errorMsg := fmt.Errorf("error reading inputs for stage: "+*id, err.Error())
		log.Errorln(logTag, errorMsg)
		return nil, false, &Error{
			Err:  errorMsg,
			Code: http.StatusBadRequest,
		}
	}
	if inputs.ParseResponseToReactivesearch != nil && *inputs.ParseResponseToReactivesearch {
		// Set the route as `_msearch` for RS API requests
		if reqCategory, ok := scriptContext.Environments["category"]; ok {
			categoryAsString, ok := reqCategory.(string)
			if ok {
				if categoryAsString == category.ReactiveSearch.String() {
					reqPath := "/" + strings.Join(indices, ",") + "/_msearch"
					inputs.Path = &reqPath
					reqMethod := "POST"
					inputs.Method = &reqMethod
					// Apply msearch header
					headers := *inputs.Headers
					headers["Content-Type"] = "application/x-ndjson"
					inputs.Headers = &headers
				}
			}
		}
	}

	// Extract params from envs
	var params = make(url.Values)
	if inputs.Params != nil {
		for k, v := range *inputs.Params {
			params.Set(k, v)
		}
	}

	formatParam := params.Get("format")

	if reqACL, ok := scriptContext.Environments["acl"]; ok {
		aclAsString, ok := reqACL.(string)
		if ok {
			// need to add check for `strings.Contains(r.URL.Path, "_cat")` because
			// ACL for root route `/` is also `Cat`.
			if aclAsString == acl.Cat.String() && strings.Contains(path, "_cat") && formatParam == "" {
				params.Add("format", "text")
			}
		}
	}
	// apply input headers
	var requestHeader = make(http.Header)
	if inputs.Headers != nil {
		for k, v := range *inputs.Headers {
			requestHeader.Set(k, v)
		}
	}
	// Suggestions fetching happens in go routine
	var popularSuggestionsWg sync.WaitGroup
	popularSuggestionsOut := make(chan suggestions.SuggestionOutput)

	var recentSuggestionsWg sync.WaitGroup
	recentSuggestionsOut := make(chan suggestions.SuggestionOutput)

	var featuredSuggestionsWg sync.WaitGroup
	featuredSuggestionsOut := make(chan suggestions.SuggestionOutput)

	var faqSuggestionsWg sync.WaitGroup
	faqSuggestionsOut := make(chan suggestions.SuggestionOutput)

	var recentDocumentSuggestionsWg sync.WaitGroup
	recentDocumentsSuggestionsOut := make(chan suggestions.SuggestionOutput)

	// fetch popular and recent suggestions
	if inputs.ParseResponseToReactivesearch != nil && *inputs.ParseResponseToReactivesearch {
		requestQuery := rsAPIRequest.Get()

		// Make sure that the requestQuery is not nil
		if requestQuery != nil {
			// fetch popular and recent suggestions
			for _, query := range requestQuery.Query {
				if query.Type == querytranslate.Suggestion {
					// fetch popular suggestions
					if query.EnablePopularSuggestions != nil &&
						*query.EnablePopularSuggestions {
						popularSuggestionsWg.Add(1)
						go func(out chan<- suggestions.SuggestionOutput) {
							defer popularSuggestionsWg.Done()
							var value string
							if query.Value != nil {
								valueAsString, ok := (*query.Value).(string)
								if ok {
									value = valueAsString
								}
							}
							config := querytranslate.PopularSuggestionsOptions{}
							if query.PopularSuggestionsConfig != nil {
								config = *query.PopularSuggestionsConfig
							}
							popularSuggestions, err := suggestions.GetPopularSuggestions(config, value, indices)
							if err != nil {
								log.Errorln(logTag, ":", err)
								out <- suggestions.SuggestionOutput{
									QueryID: *query.ID,
									Error:   err,
								}
							} else {
								out <- suggestions.SuggestionOutput{
									QueryID:     *query.ID,
									Suggestions: popularSuggestions,
									Error:       nil,
								}
							}
						}(popularSuggestionsOut)
					}

					// fetch FAQ suggestions
					if query.EnableFAQSuggestions != nil &&
						*query.EnableFAQSuggestions {
						faqSuggestionsWg.Add(1)
						go func(out chan<- suggestions.SuggestionOutput) {
							defer faqSuggestionsWg.Done()
							var value string
							if query.Value != nil {
								valueAsString, ok := (*query.Value).(string)
								if ok {
									value = valueAsString
								}
							}

							config := querytranslate.FAQSuggestionsOptions{}
							if query.FAQSuggestionsConfig != nil {
								config = *query.FAQSuggestionsConfig
							}

							faqSuggestions, err := suggestions.GetFAQSuggestions(config, value, query.SearchBoxId)
							if err != nil {
								log.Errorln(logTag, ":", err)
								out <- suggestions.SuggestionOutput{
									QueryID: *query.ID,
									Error:   err,
								}
							} else {
								out <- suggestions.SuggestionOutput{
									QueryID:     *query.ID,
									Suggestions: faqSuggestions,
									Error:       nil,
								}
							}
						}(faqSuggestionsOut)
					}

					// Determine the userId. Try to parse it from the settings else
					// fallback to the user's IP address.
					userId := ""
					ipv6, isPresent := scriptContext.Environments["ipv4"]
					if !isPresent {
						ipv4, isIpv4Present := scriptContext.Environments["ipv6"]
						if isIpv4Present {
							userId = ipv4.(string)
						}
					} else {
						userId = ipv6.(string)
					}
					if requestQuery.Settings != nil && requestQuery.Settings.UserID != nil && strings.TrimSpace(*requestQuery.Settings.UserID) != "" {
						userId = *requestQuery.Settings.UserID
					}

					// fetch recentDocumentsSuggestions
					if query.EnableDocumentSuggestions != nil && *query.EnableDocumentSuggestions && userId != "" {
						recentDocumentSuggestionsWg.Add(1)
						go func(out chan<- suggestions.SuggestionOutput, documentsConfig *querytranslate.RecentDocumentSuggestionsOptions, query querytranslate.Query) {
							defer recentDocumentSuggestionsWg.Done()
							var value string
							if query.Value != nil {
								valueAsString, ok := (*query.Value).(string)
								if ok {
									value = valueAsString
								}
							}

							if documentsConfig == nil {
								documentsConfig = &querytranslate.RecentDocumentSuggestionsOptions{}
							}

							// Make the call to get FAQ suggestions based on the query
							faqSugestions, err := suggestions.GetDocumentSuggestions(query, value, *documentsConfig, userId, indices)
							if err != nil {
								log.Errorln(logTag, ":", err)
								out <- suggestions.SuggestionOutput{
									QueryID: *query.ID,
									Error:   err,
								}
							} else {
								out <- suggestions.SuggestionOutput{
									QueryID:     *query.ID,
									Suggestions: faqSugestions,
									Error:       nil,
								}
							}
						}(recentDocumentsSuggestionsOut, query.DocumentSuggestionsOptions, query)
					}

					// fetch recent suggestions
					if query.EnableRecentSuggestions != nil &&
						*query.EnableRecentSuggestions {
						recentSuggestionsWg.Add(1)
						go func(out chan<- suggestions.SuggestionOutput) {
							defer recentSuggestionsWg.Done()
							config := querytranslate.RecentSuggestionsOptions{}
							if query.RecentSuggestionsConfig != nil {
								config = *query.RecentSuggestionsConfig
							}
							recentSuggestions, err := suggestions.GetRecentSuggestions(query, config, indices)
							if err != nil {
								log.Errorln(logTag, ":", err)
								out <- suggestions.SuggestionOutput{
									QueryID: *query.ID,
									Error:   err,
								}
							} else {
								out <- suggestions.SuggestionOutput{
									QueryID:     *query.ID,
									Suggestions: recentSuggestions,
									Error:       nil,
								}
							}
						}(recentSuggestionsOut)
					}

					featuredSuggestionsWg.Add(1)
					var value string
					if query.Value != nil {
						if valueAsString, ok := (*query.Value).(string); ok {
							value = valueAsString
						}
					}
					go func(out chan<- suggestions.SuggestionOutput) {
						defer featuredSuggestionsWg.Done()
						featuredSuggestions, err := uibuilder.GetFeaturedSuggestions(&query, value)
						if err != nil {
							log.Errorln(logTag, ":", err)
							out <- suggestions.SuggestionOutput{
								QueryID: *query.ID,
								Error:   &suggestions.Error{Error: err},
							}
						} else {
							out <- suggestions.SuggestionOutput{
								QueryID:     *query.ID,
								Suggestions: featuredSuggestions,
								Error:       nil,
							}
						}
					}(featuredSuggestionsOut)
				}
			}
		}
	}

	// GET ES URL from envs
	start := time.Now()

	finalURL := *inputs.URL

	var esResponseBody []byte
	var responseHeaders http.Header
	responseStatusCode := http.StatusOK

	// We need to check if there are any boost stages and if so
	// we will need to inject these boost stages into the m-search
	// request so that they can be made together.

	inputBodyAsStr := *inputs.Body
	inputsSplittedByLine := strings.Split(inputBodyAsStr, "\n")

	// There might be an extra new-line at the end that we will need to ignore
	isExtraNewline := false
	if inputsSplittedByLine[len(inputsSplittedByLine)-1] == "" {
		isExtraNewline = true
		inputsSplittedByLine = inputsSplittedByLine[:len(inputsSplittedByLine)-1]
	}

	boostQueriesArr := make([]map[string]interface{}, 0)

	mapContext := globalScriptContext.GetMap()
	for key := range mapContext {
		// Check if response in context is from boost stage
		if strings.HasSuffix(key, "__boost") {
			var boostResponse BoostStageResponse
			boostResponseStr, err := jsonparser.GetString(scriptContextInBytes, key)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, false, &Error{
					Err: err,
				}
			}
			err1 := json.Unmarshal([]byte(boostResponseStr), &boostResponse)
			if err1 != nil {
				log.Errorln(logTag, ":", err1)
				return nil, false, &Error{
					Err: err1,
				}
			}

			if boostResponse.ShouldSkip {
				continue
			}

			inputsSplittedByLine = append(inputsSplittedByLine, fmt.Sprintf(`{"index": "%s"}`, boostResponse.Index))
			inputsSplittedByLine = append(inputsSplittedByLine, boostResponse.Query)

			// Calculate the index position of the current query that we are adding.
			//
			// It should be the value of current inputs divided by two since the inputs
			// will always be even.
			currentIndex := (len(inputsSplittedByLine) / 2) - 1

			// Append to the boost stages array
			boostQueriesArr = append(boostQueriesArr, map[string]interface{}{
				"boostStageResponse": boostResponse,
				"queryIndex":         currentIndex,
			})
		}
	}

	// Parse the inputs back into a string
	if isExtraNewline {
		inputsSplittedByLine = append(inputsSplittedByLine, "")
	}

	*inputs.Body = strings.Join(inputsSplittedByLine, "\n")

	if len(*inputs.Body) != 0 {
		esRequest, err := http.NewRequest(*inputs.Method, finalURL, bytes.NewBuffer([]byte(*inputs.Body)))
		if err != nil {
			log.Errorln(logTag, ":", err.Error())
			return nil, false, &Error{
				Err: err,
			}
		}
		// Set path only when it is not present in URL because custom URL can contain path
		if esRequest.URL.Path == "" || esRequest.URL.Path == "/" {
			esRequest.URL.Path = *inputs.Path
		}

		// apply params
		q := esRequest.URL.Query()
		for k := range params {
			q.Set(k, params.Get(k))
		}
		esRequest.URL.RawQuery = q.Encode()

		// Set default content-type as application/json
		esRequest.Header.Set("Content-Type", "application/json")
		// apply headers
		for k := range requestHeader {
			esRequest.Header.Set(k, requestHeader.Get(k))
		}
		util.ApplyESAuth(esRequest)

		// perform Request
		log.Debugln("Pipeline Elasticsearch: REQUEST METHOD", esRequest.Method)
		log.Debugln("Pipeline Elasticsearch: REQUEST URL", esRequest.URL.String())
		log.Debugln("Pipeline Elasticsearch: REQUEST BODY", *inputs.Body)
		log.Debugln("Pipeline Elasticsearch: REQUEST HEADERS", esRequest.Header)
		response, err := util.HTTPClient().Do(esRequest)
		log.Println(fmt.Sprintf("TIME TAKEN BY ES: %dms", time.Since(start).Milliseconds()))
		if err != nil {
			log.Errorln(logTag, ": error while sending request :", reqURL, err)
			if response != nil {
				// TODO: Return ES error code
				log.Errorln(logTag, ":", err.Error())
				return nil, false, &Error{
					Err:  err,
					Code: response.StatusCode,
				}
			}
			log.Errorln(logTag, ":", err.Error())
			return nil, false, &Error{
				Err: err,
			}
		}
		defer response.Body.Close()
		responseBody, err := io.ReadAll(response.Body)
		if err != nil {
			log.Errorln(logTag, ":", err.Error())
			return nil, false, &Error{
				Err: err,
			}
		}
		esResponseBody = responseBody
		responseStatusCode = response.StatusCode
		responseHeaders = response.Header
	}

	log.Debugln("Pipeline Elasticsearch: RESPONSE", string(esResponseBody))

	// Parse the boost responses and remove them from ES response
	// so that they don't get transform into RS.
	responseIndicesToRemove := make([]int, 0)
	for queryIndex, boostMap := range boostQueriesArr {
		boostIndex, ok := boostMap["queryIndex"].(int)
		if !ok {
			continue
		}

		hitAsByte, _, _, hitParseErr := jsonparser.Get(esResponseBody, "responses", fmt.Sprintf(`[%d]`, boostIndex))
		if hitParseErr != nil {
			errMsg := fmt.Sprint("error while parsing hit for boost from msearch response: ", hitParseErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		searchResults := es7.SearchResult{}
		unmarshalErr := json.Unmarshal(hitAsByte, &searchResults)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling hit into search hit: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		boostQueriesArr[queryIndex]["searchResults"] = searchResults

		// Remove the value from ES response now
		responseIndicesToRemove = append(responseIndicesToRemove, boostIndex)
	}

	// Remove all the boost indices before proceeding
	for _, indexToRemove := range responseIndicesToRemove {
		esResponseBody = jsonparser.Delete(esResponseBody, "responses", fmt.Sprintf(`[%d]`, indexToRemove))
	}

	// transform response for RS API
	if inputs.ParseResponseToReactivesearch != nil && *inputs.ParseResponseToReactivesearch {
		// read RS API from channel
		var rsQuery = rsAPIRequest.Get()

		if rsQuery != nil {
			transformedResponse, err := querytranslate.TransformESResponse(esResponseBody, rsQuery, make([]querytranslate.MSearchDetails, 0))
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, false, &Error{
					Err: err,
				}
			}

			// Set the pipeline id in `settings` key to response
			pipelineId := scriptContext.Response.Headers[XPipelineID]
			if pipelineId != "" {
				responseBody, err := jsonparser.Set(transformedResponse, []byte(fmt.Sprintf("%q", pipelineId)), "settings", "pipelineId")
				if err != nil {
					log.Warnln(logTag, "unable to set pipelineId key in settings", err)
					responseBody2, err2 := jsonparser.Set(transformedResponse, []byte(fmt.Sprintf(`{ "pipelineId": %q }`, pipelineId)), "settings")
					if err2 != nil {
						log.Warnln(logTag, "unable to set settings key", err2)
					} else {
						transformedResponse = responseBody2
					}
				} else {
					transformedResponse = responseBody
				}
			}

			// wait for popular suggestions
			go func() {
				popularSuggestionsWg.Wait()
				close(popularSuggestionsOut)
			}()
			// popular suggestions to query ID map
			var popularSuggestionsMap = make(map[string]suggestions.SuggestionOutput)
			for result := range popularSuggestionsOut {
				popularSuggestionsMap[result.QueryID] = result
			}
			// wait for recent suggestions
			go func() {
				recentSuggestionsWg.Wait()
				close(recentSuggestionsOut)
			}()
			// recent suggestions to query ID map
			var recentSuggestionsMap = make(map[string]suggestions.SuggestionOutput)
			for result := range recentSuggestionsOut {
				recentSuggestionsMap[result.QueryID] = result
			}
			// wait for featured suggestions
			go func() {
				featuredSuggestionsWg.Wait()
				close(featuredSuggestionsOut)
			}()
			// featured suggestions to query ID map
			var featuredSuggestionsMap = make(map[string]suggestions.SuggestionOutput)
			for result := range featuredSuggestionsOut {
				featuredSuggestionsMap[result.QueryID] = result
			}

			// wait for FAQ suggestions
			go func() {
				faqSuggestionsWg.Wait()
				close(faqSuggestionsOut)
			}()

			// FAQ suggestions to query ID map
			var faqSuggestionsMap = make(map[string]suggestions.SuggestionOutput)
			for result := range faqSuggestionsOut {
				faqSuggestionsMap[result.QueryID] = result
			}

			// wait for recent document suggestions
			go func() {
				recentDocumentSuggestionsWg.Wait()
				close(recentDocumentsSuggestionsOut)
			}()

			// Document suggestions to query ID map
			var documentSuggestionsMap = make(map[string]suggestions.SuggestionOutput)
			for result := range recentDocumentsSuggestionsOut {
				documentSuggestionsMap[result.QueryID] = result
			}

			// apply suggestions
			// TODO: Update
			responseWithSuggestions, err2 := suggestions.ApplySuggestions(*rsQuery, transformedResponse, recentSuggestionsMap, popularSuggestionsMap, featuredSuggestionsMap, faqSuggestionsMap, documentSuggestionsMap)
			if err2 != nil {
				log.Errorln(logTag, ":", err2.Error)
				return nil, false, &Error{
					Err:  err2.Error,
					Code: err2.Code,
				}
			}
			esResponseBody = responseWithSuggestions

			// Apply responses from boost stages
			for _, boostQuery := range boostQueriesArr {

				boostResponse, asStageResponse := boostQuery["boostStageResponse"].(BoostStageResponse)
				if !asStageResponse {
					errMsg := fmt.Sprint("could not cast `boostStageResponse` into proper type")
					log.Errorln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err: fmt.Errorf(errMsg),
					}
				}

				// Only boost the score types
				if boostResponse.Inputs.BoostType == Promote {
					continue
				}

				searchResult, hitOk := boostQuery["searchResults"].(es7.SearchResult)
				if !hitOk {
					errMsg := fmt.Sprint("could not cast `searchHits` into es7.SearchResult")
					log.Errorln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err: fmt.Errorf(errMsg),
					}
				}

				// Check if any hits were received for the `searchResults`, if not, then ignore
				// boosting since there's nothing to boost.
				if searchResult.Hits == nil || searchResult.Hits.Hits == nil {
					log.Debug(logTag, ": Got 0 hits for boosting for query: ", boostResponse.QueryId)
					continue
				}

				hitsInBytes, valueType, _, err := jsonparser.Get(esResponseBody, boostResponse.QueryId, "hits", "hits")
				if valueType == jsonparser.NotExist {
					continue
				}
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil, false, &Error{
						Err: err,
					}
				}
				var searchHits []*es7.SearchHit
				err2 := json.Unmarshal(hitsInBytes, &searchHits)
				if err2 != nil {
					log.Errorln(logTag, ":", err2)
					return nil, false, &Error{
						Err: err2,
					}
				}

				// If type is `score` then boost accordingly
				if boostResponse.Inputs.BoostType == Score {
					for i, hit := range searchResult.Hits.Hits {
						var score float64
						boostFactor := boostResponse.Inputs.BoostFactor
						if boostFactor == 0 {
							// defaults to 1
							boostFactor = 1
						}
						if boostResponse.Inputs.BoostOperation == Add && hit.Score != nil {
							score = float64(boostFactor) + *hit.Score
						} else if boostResponse.Inputs.BoostOperation == Multiply && hit.Score != nil {
							score = float64(boostFactor) * *hit.Score
						}
						searchResult.Hits.Hits[i].Score = &score
					}
				}

				boostHits := searchResult.Hits.Hits
				boostHitsIdMap := make(map[string]interface{})
				for _, boostHit := range boostHits {
					boostHitsIdMap[boostHit.Id] = true
				}
				// Prepend boost response
				modifiedHits := boostHits
				// Apply search hits
				for _, searchHit := range searchHits {
					// Avoid merging duplicates
					if boostHitsIdMap[searchHit.Id] != true {
						modifiedHits = append(modifiedHits, searchHit)
					}
				}
				// Apply boost response as per boost inputs
				if boostResponse.Inputs.BoostType == Score && len(searchResult.Hits.Hits) > 0 {
					// Sort results by _score (decreasing order)
					for _, modifiedHit := range modifiedHits {
						var sourceData map[string]interface{}
						err := json.Unmarshal(modifiedHit.Source, &sourceData)
						if err != nil {
							log.Errorln(logTag, ":", err)
							return nil, false, &Error{
								Err: err,
							}
						}
					}
					sort.SliceStable(modifiedHits, func(i, j int) bool {
						return modifiedHits[i].Score != nil && modifiedHits[j].Score != nil && *modifiedHits[i].Score > *modifiedHits[j].Score
					})
				}
				// Maintain the size of hits
				from := 0
				size := 10
				requestQuery := rsAPIRequest.Get()
				boostQueryIndex := getBoostQueryIndex(requestQuery)
				if boostQueryIndex != -1 {
					if requestQuery.Query[boostQueryIndex].Size != nil {
						size = *requestQuery.Query[boostQueryIndex].Size
					}
					if requestQuery.Query[boostQueryIndex].From != nil {
						from = *requestQuery.Query[boostQueryIndex].From
					}
				}
				startIndex := from
				endIndex := from + size
				totalSearchHits := len(modifiedHits)
				if startIndex < totalSearchHits && endIndex < totalSearchHits {
					modifiedHits = modifiedHits[startIndex:endIndex]
				} else if startIndex < totalSearchHits {
					modifiedHits = modifiedHits[startIndex:]
				} else if endIndex < totalSearchHits {
					modifiedHits = modifiedHits[:endIndex]
				}

				marshalledHits, err4 := json.Marshal(modifiedHits)
				if err4 != nil {
					log.Errorln(logTag, ":", err4)
					return nil, false, &Error{
						Err: err4,
					}
				}
				// Set boosted hits
				responseWithBoostedHits, err5 := jsonparser.Set(esResponseBody, marshalledHits, boostResponse.QueryId, "hits", "hits")
				if err5 != nil {
					log.Errorln(logTag, ":", err5)
					return nil, false, &Error{
						Err: err5,
					}
				}
				// Updated final response
				esResponseBody = responseWithBoostedHits
			}

			// Boost promote type of hits now
			for _, boostQuery := range boostQueriesArr {
				boostResponse, asStageResponse := boostQuery["boostStageResponse"].(BoostStageResponse)
				if !asStageResponse {
					errMsg := fmt.Sprint("could not cast `boostStageResponse` into proper type")
					log.Errorln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err: fmt.Errorf(errMsg),
					}
				}

				// Scores will be boosted in the above loop.
				if boostResponse.Inputs.BoostType == Score {
					continue
				}

				searchResult, hitOk := boostQuery["searchResults"].(es7.SearchResult)
				if !hitOk {
					errMsg := fmt.Sprint("could not cast `searchHits` into es7.SearchResult")
					log.Errorln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err: fmt.Errorf(errMsg),
					}
				}

				hitsInBytes, valueType, _, err := jsonparser.Get(esResponseBody, boostResponse.QueryId, "hits", "hits")
				if valueType == jsonparser.NotExist {
					continue
				}
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil, false, &Error{
						Err: err,
					}
				}
				var searchHits []*es7.SearchHit
				err2 := json.Unmarshal(hitsInBytes, &searchHits)
				if err2 != nil {
					log.Errorln(logTag, ":", err2)
					return nil, false, &Error{
						Err: err2,
					}
				}

				boostHits := searchResult.Hits.Hits
				boostHitsIdMap := make(map[string]interface{})
				for _, boostHit := range boostHits {
					boostHitsIdMap[boostHit.Id] = true
				}
				// Prepend boost response
				modifiedHits := boostHits
				// Apply search hits
				for _, searchHit := range searchHits {
					// Avoid merging duplicates
					if boostHitsIdMap[searchHit.Id] != true {
						modifiedHits = append(modifiedHits, searchHit)
					}
				}

				marshalledHits, err4 := json.Marshal(modifiedHits)
				if err4 != nil {
					log.Errorln(logTag, ":", err4)
					return nil, false, &Error{
						Err: err4,
					}
				}
				// Set boosted hits
				responseWithBoostedHits, err5 := jsonparser.Set(esResponseBody, marshalledHits, boostResponse.QueryId, "hits", "hits")
				if err5 != nil {
					log.Errorln(logTag, ":", err5)
					return nil, false, &Error{
						Err: err5,
					}
				}
				// Updated final response
				esResponseBody = responseWithBoostedHits
			}
		}
	}
	// Set response to script context
	// set headers
	for k := range responseHeaders {
		if k != "Content-Length" {
			scriptContext.Response.Headers[k] = responseHeaders.Get(k)
		}
	}

	independentResponse := make(map[string]interface{})

	// Extract the independent requests from the context.
	if len(*inputs.IndependentBody) != 0 {
		// Unmarshal the content into an array of map[string]interface{}
		independentReqBody := make([]map[string]interface{}, 0)
		unmarshalErr := json.Unmarshal([]byte(*inputs.IndependentBody), &independentReqBody)
		if unmarshalErr != nil {
			log.Warnln(logTag, ": error while unmarshalling independent requests: ", unmarshalErr)
			return nil, false, &Error{
				Err: unmarshalErr,
			}
		}

		hostAsString := scriptContext.Environments["origin"].(string)
		isTlsAsBool := scriptContext.Environments["isTLS"].(bool)

		for _, independentReq := range independentReqBody {
			// Make the request with the passed details.
			requestId := independentReq["id"].(string)

			respBody, _, reqErr := querytranslate.ExecuteIndependentQuery(independentReq, hostAsString, isTlsAsBool, scriptContext.Request.Headers)

			if reqErr != nil {
				log.Warnln(logTag, ": ", reqErr)
				return nil, false, &Error{
					Err: reqErr,
				}
			}

			// TODO: Decide whether to map the response to the ID or extract the body
			// for the ID from RS response and use that instead?
			responseAsInterface := new(map[string]interface{})
			unmarshalIndependentResponseErr := json.Unmarshal(respBody, &responseAsInterface)
			if unmarshalIndependentResponseErr != nil {
				errMsg := fmt.Sprintf("error while unmarshalling received response for independent request with ID: `%s` and err: `%v`", requestId, unmarshalIndependentResponseErr)
				log.Errorln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err: fmt.Errorf(errMsg),
				}
			}

			independentResponse[requestId] = responseAsInterface
		}

		if len(independentResponse) > 0 {
			// Unmarshal the stage 1 response into a map and merge the independent
			// responses as well
			rsResponseAsMap := make(map[string]interface{})
			rsResponseAsMapErr := json.Unmarshal(esResponseBody, &rsResponseAsMap)
			if rsResponseAsMapErr != nil {
				errMsg := fmt.Sprint("error while unmarshalling RS response into a map to modify it: ", rsResponseAsMapErr)
				log.Errorln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err: fmt.Errorf(errMsg),
				}
			}

			// Merge the independent responses into the final response
			for id, response := range independentResponse {
				rsResponseAsMap[id] = response
			}

			// Marshal the map back into bytes with the updated
			// content.
			var marshalErr error
			esResponseBody, marshalErr = json.Marshal(rsResponseAsMap)

			if marshalErr != nil {
				errMsg := fmt.Sprint("error while marshalling rs response back into bytes from modified map: ", marshalErr)
				log.Errorln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err: fmt.Errorf(errMsg),
				}
			}
		}
	}

	// replace index to alias
	for _, index := range indices {
		alias := classify.GetIndexAlias(index)
		if alias != "" {
			esResponseBody = bytes.Replace(esResponseBody, []byte(`"`+index+`"`), []byte(`"`+alias+`"`), -1)
			continue
		}
		// if alias is present in url get index name from cache
		indexName := classify.GetAliasIndex(index)
		if indexName != "" {
			esResponseBody = bytes.Replace(esResponseBody, []byte(`"`+indexName+`"`), []byte(`"`+index+`"`), -1)
		}
	}
	var output interface{}
	if async {
		// write output to a top-level variable
		output = map[string]interface{}{
			*id: string(esResponseBody),
		}
	} else {
		// set response code
		scriptContext.Response.Code = responseStatusCode
		scriptContext.Response.Body = string(esResponseBody)
		scriptContext.Response.Headers["X-Origin"] = "reactivesearch.io"

		// Set request values
		scriptContext.Request.Body = *inputs.Body
		scriptContext.Request.URL = finalURL
		scriptContext.Request.Method = *inputs.Method

		requestHeaders := make(map[string]string)
		requestHeaders["Content-Type"] = "application/json"
		for key, value := range requestHeader {
			requestHeaders[key] = strings.Join(value, ", ")
		}
		scriptContext.Request.Headers = requestHeaders

		output = scriptContext
	}

	responseInBytes, err := json.Marshal(scriptContext.Response)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}

	if inputs.SetResponseToKV != nil {
		errStoringInKV := rules.StoreValueInCacheWithObject(*inputs.SetResponseToKV, string(responseInBytes), cache.GetSearchCache())
		if errStoringInKV != nil {
			log.Errorln(logTag, ":", errStoringInKV)
			return nil, false, &Error{
				Err: errStoringInKV,
			}
		}
	}

	contextInBytes, err := json.Marshal(output)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}
