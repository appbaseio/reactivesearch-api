package pipelines

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	"github.com/appbaseio-confidential/reactivesearch/plugins/suggestions"
	"github.com/appbaseio-confidential/reactivesearch/plugins/uibuilder"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
)

type ZincInput struct {
	Protocol        *string            `json:"protocol,omitempty" jsonschema:"title=Protocol" jsonschema_description:"Protocol to use for connecting. Defaults to 'http'."`
	Host            *string            `json:"host,omitempty" jsonschema:"title=Host" jsonschema_description:"Host name and port for Zinc. For example: 'localhost:4080'"`
	URL             *string            `json:"url,omitempty" jsonschema:"title=Request URL" jsonschema_description:"Zinc URL, for e.g: 'https://admin:Complexpass#123@zinc-url.com'. The URL can have path and query params too, for instance, 'https://zinc-url.com/_cat/indices?format=JSON'."`
	Headers         *map[string]string `json:"headers,omitempty" jsonschema:"title=Headers" jsonschema_description:"Headers to be passed in the Zinc request."`
	Credentials     *string            `json:"credentials,omitempty" jsonschema:"title=Credentials" jsonschema_description:"Credentials to access the Zinc host. Example: 'username:password'"`
	IndependentBody *string            `json:"independentBody,omitempty" jsonschema:"title=Independent Body" jsonschema_description:"Array of independent requests, i:e ones that contain the endpoint in the query."`
}

// GetZincInputSchema will return the schema for Zinc
func GetZincInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&ZincInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeZincStage will execute the Zinc request and return
// the response after parsing it back to ReactiveSearch equivalent.
func executeZincStage(
	stage ESPipelineStage,
	parsedInputs *string,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	id := getStageID(stage)
	scriptContextInBytes := globalScriptContext.Get()
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}

	// Verify the user inputs
	//
	// If inputs are empty, we just initialize a new
	// input struct.
	if parsedInputs == nil {
		defaultInputs := "{}"
		parsedInputs = &defaultInputs
	}

	// Parse the inputs to ZincInput
	var inputs ZincInput
	inputParseErr := json.Unmarshal([]byte(*parsedInputs), &inputs)
	if inputParseErr != nil {
		log.Warnln(logTag, ": error while parsing inputs to Zinc input, ", inputParseErr)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Error while trying to parse inputs as Zinc ones: %s", inputParseErr),
			Code: http.StatusBadRequest,
		}
	}

	// If headers are not passed, initialize an empty
	// map
	if inputs.Headers == nil {
		defaultHeaders := make(map[string]string)
		inputs.Headers = &defaultHeaders
	}

	// If URL is not passed and host etc are also not passed
	// we need to set the default values.
	if inputs.URL == nil && inputs.Host == nil {
		zincClient := util.GetZincClient()
		msearchURL := fmt.Sprintf("%s/%s", zincClient.URL, "es/_msearch")
		inputs.URL = &msearchURL
		credentials := fmt.Sprintf("%s:%s", zincClient.Username, zincClient.Password)
		inputs.Credentials = &credentials
	}

	if inputs.Protocol == nil {
		defaultProtocol := "http"
		inputs.Protocol = &defaultProtocol
	}

	if inputs.Credentials != nil {
		(*inputs.Headers)["Authorization"] = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(*inputs.Credentials)))
	}

	// If `host` is passed then build the URL using it
	if inputs.Host != nil {
		urlToUse := fmt.Sprintf("%s://%s/es/_msearch", *inputs.Protocol, *inputs.Host)
		inputs.URL = &urlToUse
	}

	if inputs.IndependentBody == nil {
		independentReqs, isPresent := scriptContext.Environments["INDEPENDENT_REQUESTS"]
		if isPresent {
			reqsAsStr := independentReqs.(string)
			inputs.IndependentBody = &reqsAsStr
		}
	}

	// Extract the indices
	indices := make([]string, 0)
	indicesInterface, ok := scriptEnvs["index"].([]interface{})
	if ok {
		for _, index := range indicesInterface {
			indexAsString, ok := index.(string)
			if ok {
				indices = append(indices, indexAsString)
			}
		}
	}

	// Handle fetching suggestions
	var popularSuggestionsWg sync.WaitGroup
	popularSuggestionsOut := make(chan suggestions.SuggestionOutput)

	var recentSuggestionsWg sync.WaitGroup
	recentSuggestionsOut := make(chan suggestions.SuggestionOutput)

	var featuredSuggestionsWg sync.WaitGroup
	featuredSuggestionsOut := make(chan suggestions.SuggestionOutput)

	requestQuery := rsAPIRequest.Get()
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

	// Run the zinc query now
	zincBodyReceived, _, zincRunErr := runZincQuery(inputs.URL, []byte(scriptContext.Request.Body), *inputs.Headers)

	if zincRunErr != nil {
		errMsg := fmt.Sprintf("error while running the zinc request and hitting upstream: %s", zincRunErr.Err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, zincRunErr
	}

	var output interface{}

	// Marshal the output into JSON
	outputAsJSON, parseErr := parseZincToReactiveSearch(zincBodyReceived, rsAPIRequest.Get().Query)
	if parseErr != nil {
		errMsg := fmt.Sprintf("error while parsing zinc to RS equivalent: %s", parseErr.Err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, parseErr
	}

	// read RS API from channel
	var rsQuery = rsAPIRequest.Get()

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

	independentResponse := make(map[string]interface{})

	// Extract the independent requests from the context.
	if inputs.IndependentBody != nil && len(*inputs.IndependentBody) != 0 {
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
			rsResponseAsMapErr := json.Unmarshal(outputAsJSON, &rsResponseAsMap)
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
			outputAsJSON, marshalErr = json.Marshal(rsResponseAsMap)

			if marshalErr != nil {
				errMsg := fmt.Sprint("error while marshalling rs response back into bytes from modified map: ", marshalErr)
				log.Errorln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err: fmt.Errorf(errMsg),
				}
			}
		}
	}

	responseWithSuggestions, applyErr := suggestions.ApplySuggestions(*rsQuery, outputAsJSON, recentSuggestionsMap, popularSuggestionsMap, featuredSuggestionsMap, nil, nil)
	if applyErr != nil {
		log.Errorln(logTag, ":", applyErr.Error)
		return nil, false, &Error{
			Err:  applyErr.Error,
			Code: applyErr.Code,
		}
	}
	outputAsJSON = responseWithSuggestions

	// Set the pipeline id in `settings` key to response
	pipelineId := scriptContext.Response.Headers[XPipelineID]
	if pipelineId != "" {
		responseBody, err := jsonparser.Set(outputAsJSON, []byte(fmt.Sprintf("%q", pipelineId)), "settings", "pipelineId")
		if err != nil {
			log.Warnln(logTag, "unable to set pipelineId key in settings", err)
			responseBody2, err2 := jsonparser.Set(outputAsJSON, []byte(fmt.Sprintf(`{ "pipelineId": %q }`, pipelineId)), "settings")
			if err2 != nil {
				log.Warnln(logTag, "unable to set settings key", err2)
			} else {
				outputAsJSON = responseBody2
			}
		} else {
			outputAsJSON = responseBody
		}
	}

	// If async then set the output in the context, else
	// update the response body.
	if async {
		output = map[string]interface{}{
			*id: string(outputAsJSON),
		}
	} else {
		scriptContext.Response.Body = string(outputAsJSON)
		scriptContext.Response.Code = http.StatusOK
		if scriptContext.Response.Headers == nil {
			scriptContext.Response.Headers = make(map[string]string)
		}
		scriptContext.Response.Headers["X-Origin"] = "reactivesearch.io"

		// Set request values
		// NOTE: Body is not changed in this stage since we are directly
		// using the context body.
		scriptContext.Request.URL = *inputs.URL
		scriptContext.Request.Headers = *inputs.Headers
		scriptContext.Request.Method = http.MethodPost

		output = scriptContext
	}

	contextInBytes, marshalErr := json.Marshal(output)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling output, ", marshalErr)
		log.Errorln(logTag, ": ", errMsg)

		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	return contextInBytes, false, nil

}

// runZincQuery will run the zinc query based on the passed inputs
// and the request body. It will return the response for the body
// passed.
func runZincQuery(uri *string, requestBody []byte, headers map[string]string) ([]byte, *http.Response, *Error) {
	request, reqCreateErr := http.NewRequest(http.MethodPost, *uri, bytes.NewReader(requestBody))
	if reqCreateErr != nil {
		errMsg := fmt.Sprint("error while creating the request to send Zinc query, ", reqCreateErr)
		log.Errorln(logTag, ": ", errMsg)

		return nil, nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// Parse headers and add to the request
	for key, value := range headers {
		request.Header.Add(key, value)
	}

	response, reqErr := util.HTTPClient().Do(request)
	if reqErr != nil {
		errMsg := fmt.Sprint("error while sending request to Zinc, ", reqErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// Read the body.
	responseInBytes, readErr := ioutil.ReadAll(response.Body)
	if readErr != nil {
		errMsg := fmt.Sprint("error while reading the response body from Zinc, ", readErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, response, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	return responseInBytes, response, nil
}

// parseZincToReactiveSearch will parse the zinc response to it's
// ReactiveSearch equivalent.
func parseZincToReactiveSearch(zincResponse []byte, allQueries []querytranslate.Query) ([]byte, *Error) {
	// Parse the zinc response into an array of interface
	zincAsMap := make(map[string]interface{}, 0)
	unmarshalErr := json.Unmarshal(zincResponse, &zincAsMap)
	if unmarshalErr != nil {
		errMsg := fmt.Sprintf("error while unmarshalling zinc body into an array of interface to parse it: %s", unmarshalErr.Error())
		return nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	zincAsArr, asArrOk := zincAsMap["responses"].([]interface{})
	if !asArrOk {
		errMsg := fmt.Sprintf("error while parsing `responses` to an array")
		log.Warnln(logTag, ": ", errMsg)
		return nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// allExecutedQueries will be the queries that were actually executed
	// so that they can be extracted properly.
	allExecutedQueries := make([]querytranslate.Query, 0)
	for _, queryEach := range allQueries {
		// Skip fetching queries that have execute as false
		if (queryEach.Execute != nil && !*queryEach.Execute) || ((queryEach.EnableEndpointSuggestions == nil || *queryEach.EnableEndpointSuggestions) && queryEach.Endpoint != nil) {
			continue
		}

		allExecutedQueries = append(allExecutedQueries, queryEach)
	}

	maxTook := 0
	parsedResponse := make(map[string]interface{})
	for queryIndex, queryEach := range allExecutedQueries {
		responseAsMap, asMapOk := zincAsArr[queryIndex].(map[string]interface{})
		if !asMapOk {
			errMsg := fmt.Sprintf("error while parsing zinc response at index `%d` to a map", queryIndex)
			log.Warnln(logTag, ": ", errMsg)
			return nil, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}
		tookAsInt, asIntErr := responseAsMap["took"].(float64)
		if !asIntErr {
			maxTook = int(tookAsInt)
		}

		// Remove excluded fields from the source
		if queryEach.ExcludeFields != nil {
			responseAsMap = parseExcludeFields(responseAsMap, *queryEach.ExcludeFields)
		}

		// If indexSuggestions is disabled, then return an empty hits object.
		// NOTE: Following behavior should be changed when other suggestion
		// types are supported.
		if queryEach.EnableIndexSuggestions != nil && *queryEach.EnableIndexSuggestions {
			var suggestionExtractErr error
			responseAsMap, suggestionExtractErr = extractIndexSuggestionsForZinc(responseAsMap, queryEach)
			if suggestionExtractErr != nil {
				errMsg := fmt.Sprint("error while extracting the suggestions from the response, ", suggestionExtractErr)
				log.Errorln(logTag, ": ", errMsg)

				return nil, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusInternalServerError,
				}
			}
		}

		// If the type was suggestion and enableIndexSuggestions or enableEndpointSuggestions
		// was passed as false then we need to remove the error which will probably be
		// indicating that the `index` is not present.
		if queryEach.Type == querytranslate.Suggestion && ((queryEach.EnableIndexSuggestions != nil &&
			!*queryEach.EnableIndexSuggestions) || (queryEach.EnableEndpointSuggestions != nil &&
			!*queryEach.EnableEndpointSuggestions)) {
			delete(responseAsMap, "error")
		}

		parsedResponse[*queryEach.ID] = responseAsMap
	}

	parsedResponse["settings"] = map[string]interface{}{"took": maxTook}

	marshaledBody, marshalErr := json.Marshal(parsedResponse)
	if marshalErr != nil {
		return nil, &Error{
			Err:  marshalErr,
			Code: http.StatusInternalServerError,
		}
	}

	return marshaledBody, nil
}

// parseExcludeFields will parse the passed response and update
// all the hits to remove the excludeFields from the `_source`
// map.
//
// This function will return an update map with the changes.
func parseExcludeFields(responseMap map[string]interface{}, excludeFields []string) map[string]interface{} {
	// If `hits.hits` are not present, don't throw any error.
	responseHits, topHitsOk := responseMap["hits"]
	if !topHitsOk {
		log.Warnln(logTag, ": ", "`response.hits` not present in map")
		return responseMap
	}

	topHitsAsMap, asMapOk := responseHits.(map[string]interface{})
	if !asMapOk {
		log.Warnln(logTag, ": `response.hits` cannot be parsed into a map")
		return responseMap
	}

	nestedHits, nestedHitOk := topHitsAsMap["hits"]
	if !nestedHitOk {
		log.Warnln(logTag, ": ", "`response.hits.hits` not present in response")
		return responseMap
	}

	nestedHitsAsMap, asMapOk := nestedHits.([]interface{})
	if !asMapOk {
		log.Warnln(logTag, ": ", "`response.hits.hits` not an array")
		return responseMap
	}

	for hitIndex, hitEach := range nestedHitsAsMap {
		hitAsMap := hitEach.(map[string]interface{})
		source, sourcePresent := hitAsMap["_source"]
		if !sourcePresent {
			continue
		}

		sourceAsMap, asMapOk := source.(map[string]interface{})
		if !asMapOk {
			continue
		}

		for _, key := range excludeFields {
			delete(sourceAsMap, key)
		}

		// Update the source now
		hitAsMap["_source"] = sourceAsMap
		nestedHitsAsMap[hitIndex] = hitAsMap
	}

	topHitsAsMap["hits"] = nestedHitsAsMap
	responseMap["hits"] = topHitsAsMap
	return responseMap
}

// extractIndexSuggestionsForZinc will extract the index suggestions
// for Zinc.
func extractIndexSuggestionsForZinc(responseMap map[string]interface{}, rsQuery querytranslate.Query) (map[string]interface{}, error) {
	// If `hits.hits` are not present, don't throw any error.
	responseHits, topHitsOk := responseMap["hits"]
	if !topHitsOk {
		errMsg := "`response.hits` not present in map"
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	topHitsAsMap, asMapOk := responseHits.(map[string]interface{})
	if !asMapOk {
		errMsg := "`response.hits` cannot be parsed into a map"
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	nestedHits, nestedHitOk := topHitsAsMap["hits"]
	if !nestedHitOk {
		errMsg := "`response.hits.hits` not present in response"
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	nestedHitsAsMap, asMapOk := nestedHits.([]interface{})
	if !asMapOk {
		errMsg := "`response.hits.hits` not an array"
		log.Warnln(logTag, ": ", errMsg)
		return nil, fmt.Errorf(errMsg)
	}

	// Convert the hits into an ESDoc array
	esDocHits := make([]querytranslate.ESDoc, 0)
	for _, hit := range nestedHitsAsMap {
		hitAsMap, ok := hit.(map[string]interface{})
		if !ok {
			return nil, errors.New("error while parsing hit to map")
		}

		hitAsBytes, err := json.Marshal(hitAsMap)
		if err != nil {
			return nil, errors.New(fmt.Sprint("error while marshalling hit, ", err))
		}

		// Unmarshal into ESDoc now.
		var esDoc querytranslate.ESDoc
		unmarshallErr := json.Unmarshal(hitAsBytes, &esDoc)
		if unmarshallErr != nil {
			return nil, errors.New(fmt.Sprint("error while unmarshalling marshalled hit into ESDoc, ", unmarshallErr))
		}

		esDocHits = append(esDocHits, esDoc)
	}

	// Set value if not present
	if rsQuery.Value == nil {
		var defaultValue interface{}
		defaultValue = "*"
		rsQuery.Value = &defaultValue
	}

	valueAsString, ok := (*rsQuery.Value).(string)
	if !ok {
		return nil, errors.New("value should be a string for suggestion to be parsed")
	}

	// Check if `indexSuggestionsConfig.valueFields` is passed and if so
	// then give it priority over
	var valueFields = []string{}
	if rsQuery.IndexSuggestionsConfig != nil && rsQuery.IndexSuggestionsConfig.ValueFields != nil {
		valueFields = *rsQuery.IndexSuggestionsConfig.ValueFields
	}

	var normalizedDataFields = []string{}
	normalizedFields := querytranslate.NormalizedDataFields(rsQuery.DataField, rsQuery.FieldWeights)
	for _, dataField := range normalizedFields {
		normalizedDataFields = append(normalizedDataFields, dataField.Field)
	}
	suggestionsConfig := querytranslate.SuggestionsConfig{
		// Fields to extract suggestions
		DataFields: normalizedDataFields,
		// Query value
		Value:                       valueAsString,
		ShowDistinctSuggestions:     rsQuery.ShowDistinctSuggestions,
		EnablePredictiveSuggestions: rsQuery.EnablePredictiveSuggestions,
		MaxPredictedWords:           rsQuery.MaxPredictedWords,
		EnableSynonyms:              rsQuery.EnableSynonyms,
		ApplyStopwords:              rsQuery.ApplyStopwords,
		Stopwords:                   rsQuery.Stopwords,
		URLField:                    rsQuery.URLField,
		CategoryField:               rsQuery.CategoryField,
		HighlightField:              rsQuery.HighlightField,
		HighlightConfig:             rsQuery.HighlightConfig,
		Language:                    rsQuery.SearchLanguage,
		IndexSuggestionsConfig:      rsQuery.IndexSuggestionsConfig,
		ValueFields:                 valueFields,
	}

	suggestionHits := querytranslate.GetIndexSuggestions(suggestionsConfig, esDocHits)

	// Set the suggestionHits in `hits.hits` now and return the responseMap
	topHitsAsMap["hits"] = suggestionHits
	responseMap["hits"] = topHitsAsMap

	return responseMap, nil
}
