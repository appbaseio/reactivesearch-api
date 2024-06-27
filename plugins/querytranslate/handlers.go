package querytranslate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/model/index"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/appbaseio-confidential/reactivesearch/util/iplookup"
	"github.com/buger/jsonparser"
	"github.com/gorilla/mux"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func (r *QueryTranslate) search() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		vars := mux.Vars(req)
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()
		rsAPIRequest, err := FromContext(req.Context())
		if err != nil {
			msg := "error occurred while retrieving request body from context"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		// Extract the msearch details
		mSearchDetails, fetchErr := FromMSearchDetailsRequestContext(req.Context())
		if fetchErr != nil {
			msg := "error occurred while retrieving msearch details body from context"
			log.Errorln(logTag, ":", fetchErr)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		// If msearch details are not present, throw error
		if mSearchDetails == nil {
			msg := "msearch details not found in context"
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		var esResponseBody []byte
		responseStatusCode := http.StatusOK
		if len(reqBody) != 0 {
			reqURL := "/" + vars["index"] + "/_msearch"
			start := time.Now()
			httpRes, err := makeESRequest(ctx, reqURL, http.MethodPost, reqBody, req.URL.Query())
			if err != nil {
				msg := err.Error()
				log.Errorln(logTag, ":", err)
				// Response can be nil sometimes
				if httpRes != nil {
					util.WriteBackError(w, msg, httpRes.StatusCode)
					return
				}
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			log.Println("TIME TAKEN BY ES:", time.Since(start))
			if httpRes.StatusCode > 500 {
				msg := "unable to connect to the upstream Elasticsearch cluster"
				log.Errorln(logTag, ":", msg)
				util.WriteBackError(w, msg, httpRes.StatusCode)
				return
			}
			esResponseBody = httpRes.Body
			responseStatusCode = httpRes.StatusCode
		}
		rsResponse, err := TransformESResponse(esResponseBody, rsAPIRequest, *mSearchDetails)
		if err != nil {
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error getting the index names from context"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		// Add support for AIAnswer related functionality.
		//
		// We will have to iterate the RSQuery body and find out which requests
		// have the `enableAI` field true as well as value is present.
		//
		// Execute the AI Answer functionality only if the response returned OK from ES
		if responseStatusCode == http.StatusOK {
			updatedRSResponse, AIAnswerErr := ExecuteAIAnswerInQuery(rsAPIRequest, rsResponse, indices, r.sessionMap, iplookup.FromRequest(req))
			if AIAnswerErr != nil {
				log.Warnln(logTag, ": error while executing AI Answer query: ", AIAnswerErr.Error())
				telemetry.WriteBackErrorWithTelemetry(req, w, "error executing AI Answer query: "+AIAnswerErr.Error(), http.StatusBadRequest)
				return
			}
			rsResponse = updatedRSResponse
		}

		// This is where the independent requests will be done.
		independentReqBody, independentErr := FromIndependentRequestContext(req.Context())
		if independentErr != nil {
			log.Errorln(logTag, ": ", err)
			util.WriteBackError(w, "Can't read independent requests built", http.StatusBadRequest)
			return
		}

		independentResponse := make(map[string]interface{})

		for _, independentReq := range *independentReqBody {
			// Make the request with the passed details.
			requestId := independentReq["id"].(string)

			// Parse the request headers
			requestHeaders := make(map[string]string)
			for key, value := range req.Header {
				requestHeaders[key] = strings.Join(value, "")
			}

			respBody, _, reqErr := ExecuteIndependentQuery(independentReq, req.Host, req.TLS != nil, requestHeaders)

			if reqErr != nil {
				log.Warnln(logTag, ": ", reqErr)
				util.WriteBackError(w, reqErr.Error(), http.StatusInternalServerError)
				return
			}

			// TODO: Decide whether to map the response to the ID or extract the body
			// for the ID from RS response and use that instead?

			responseAsInterface := new(map[string]interface{})
			unmarshalIndependentResponseErr := json.Unmarshal(respBody, &responseAsInterface)
			if unmarshalIndependentResponseErr != nil {
				errMsg := fmt.Sprintf("error while unmarshalling received response for independent request with ID: `%s` and err: `%v`", requestId, unmarshalIndependentResponseErr)
				log.Errorln(logTag, ": ", errMsg)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			independentResponse[requestId] = responseAsInterface
		}

		if len(independentResponse) > 0 {
			// Unmarshal the stage 1 response into a map and merge the independent
			// responses as well
			rsResponseAsMap := make(map[string]interface{})
			rsResponseAsMapErr := json.Unmarshal(rsResponse, &rsResponseAsMap)
			if rsResponseAsMapErr != nil {
				errMsg := fmt.Sprint("error while unmarshalling RS response into a map to modify it: ", rsResponseAsMapErr)
				log.Errorln(logTag, ": ", errMsg)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			// Merge the independent responses into the final response
			for id, response := range independentResponse {
				rsResponseAsMap[id] = response
			}

			// Marshal the map back into bytes with the updated
			// content.
			var marshalErr error
			rsResponse, marshalErr = json.Marshal(rsResponseAsMap)

			if marshalErr != nil {
				errMsg := fmt.Sprint("error while marshalling rs response back into bytes from modified map: ", marshalErr)
				log.Errorln(logTag, ": ", errMsg)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
		}

		// Replace indices to alias
		for _, index := range indices {
			alias := classify.GetIndexAlias(index)
			if alias != "" {
				rsResponse = bytes.Replace(rsResponse, []byte(`"`+index+`"`), []byte(`"`+alias+`"`), -1)
				continue
			}
			// if alias is present in url get index name from cache
			indexName := classify.GetAliasIndex(index)
			if indexName != "" {
				rsResponse = bytes.Replace(rsResponse, []byte(`"`+indexName+`"`), []byte(`"`+index+`"`), -1)
			}
		}
		// if status code is not 200 write rsResponse otherwise return raw response from ES
		// avoid copy for performance reasons
		if responseStatusCode == http.StatusOK {
			util.WriteBackRaw(w, rsResponse, responseStatusCode)
		} else {
			util.WriteBackRaw(w, esResponseBody, responseStatusCode)
		}
	}
}

// ExecuteIndependentQuery will execute the passed independent query and return
// the response in bytes, HTTP response and error (if any).
func ExecuteIndependentQuery(independentReq map[string]interface{}, host string, isTLS bool, reqHeaders map[string]string) ([]byte, *http.Response, error) {
	requestId := independentReq["id"].(string)

	endpointAsMap, endpointAsMapOk := independentReq["endpoint"].(map[string]interface{})
	if !endpointAsMapOk {
		errMsg := fmt.Sprint("error while converting endpoint to map for independent request with ID: ", requestId)
		return nil, nil, fmt.Errorf(errMsg)
	}

	urlToHit, urlOk := endpointAsMap["url"].(string)
	if !urlOk {
		errMsg := fmt.Sprint("error while extracting URL from independent request built for: ", requestId)
		return nil, nil, fmt.Errorf(errMsg)
	}

	// If the URL is not complete, append the host to make it complete.
	if !strings.HasPrefix(urlToHit, "http") && strings.HasPrefix(urlToHit, "/") {
		// Add the host to the path
		scheme := "http"
		if isTLS {
			scheme = "https"
		}
		urlToHit = fmt.Sprintf("%s://%s%s", scheme, host, urlToHit)
	}

	methodToUse, methodOk := endpointAsMap["method"].(string)
	if !methodOk {
		errMsg := fmt.Sprint("error while extracting method from independent request built for: ", requestId)
		return nil, nil, fmt.Errorf(errMsg)
	}

	headersToUse, headerOk := endpointAsMap["headers"].(map[string]interface{})
	if !headerOk {
		errMsg := fmt.Sprint("error while extracting headers from independent request built for: ", requestId)
		return nil, nil, fmt.Errorf(errMsg)
	}
	headerToSend := make(http.Header)
	isAuthPresent := false

	for key, value := range headersToUse {
		valueAsString, valueAsStrOk := value.(string)

		if strings.ToLower(key) == "authorization" {
			isAuthPresent = true
		}

		if !valueAsStrOk {
			errMsg := fmt.Sprintf("error while converting header value to string for key `%s` and request: `%s`", key, requestId)
			log.Warnln(logTag, ": ", errMsg)
			return nil, nil, fmt.Errorf(errMsg)
		}
		headerToSend.Set(key, valueAsString)
	}

	// If auth header was not passed by user, we can try to extract
	// it from the request headers.
	if !isAuthPresent {
		// Go through request headers and if authorization is present
		// then pass it accordingly.
		for key, value := range reqHeaders {
			if strings.ToLower(key) == "authorization" {
				headerToSend.Set(key, value)
				break
			}
		}
	}

	bodyToUse, bodyOk := endpointAsMap["body"].(interface{})
	if !bodyOk {
		errMsg := fmt.Sprint("error while extracting body from independent request built for: ", requestId)
		log.Warnln(logTag, ": ", errMsg)
		// No need to return, instead set the body as empty
		defaultBody := make([]byte, 0)
		bodyToUse = defaultBody
	}

	// Marshal the body
	bodyInBytes, marshalErr := json.Marshal(bodyToUse)
	if marshalErr != nil {
		errMsg := fmt.Sprintf("error while marshalling body to send it for independent request for request `%s` with err: %v", requestId, marshalErr)
		log.Errorln(logTag, ": ", errMsg)
		return nil, nil, fmt.Errorf(errMsg)
	}

	respBody, res, reqErr := util.MakeRequestWithHeader(urlToHit, methodToUse, bodyInBytes, headerToSend)
	if reqErr != nil {
		errMsg := fmt.Sprintf("error while sending independent request for ID: `%s` with err: `%v`", requestId, reqErr)
		log.Errorln(logTag, ": ", errMsg)
		return nil, nil, fmt.Errorf(errMsg)
	}

	// Parse the response and if it is of RS structure, return the top level of
	// the response instead of returning a nested level.
	responseToReturn, respErr := RemoveEndpointRecursionIfRS(respBody, requestId)
	if respErr != nil {
		errMsg := fmt.Sprint("error while parsing the response to remove recursion, ", respErr)
		log.Errorln(logTag, ": ", errMsg)
		return nil, nil, fmt.Errorf(errMsg)
	}

	return responseToReturn, res, reqErr
}

func (r *QueryTranslate) validate() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}

		// Request body is nd-json so we need to convert it into an
		// array of strings by splitting on \n
		reqBodySplitted := strings.Split(string(reqBody), "\n")

		// Remove the last item since it's empty
		if len(reqBodySplitted) > 0 {
			reqBodySplitted = reqBodySplitted[:len(reqBodySplitted)-1]
		}

		// Extract the headers passed with the current request without the
		// NOTE: Authorization header will be removed at the end before
		// returning the response.
		headersPassed := make(map[string]interface{})
		for key, value := range req.Header {
			headersPassed[key] = strings.Join(value, ", ")
		}

		// Extract the reqBody into the required format that shows based on ID.

		// Extract some request details that might be required later
		vars := mux.Vars(req)
		defaultURL := fmt.Sprint(util.GetESURL(), "/", vars["index"], "/_search")
		request, err := http.NewRequest("POST", defaultURL, nil)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Error while creating request", http.StatusBadRequest)
			return
		}
		filteredParams := req.URL.Query()
		for k := range filteredParams {
			if k == "preference" {
				filteredParams.Del("preference")
			}
		}
		request.URL.RawQuery = filteredParams.Encode()
		methodUsed := req.Method

		validateMapToShow, parseErr := ParseMsearchToValidate(reqBodySplitted, request.URL.String(), methodUsed, headersPassed)
		if parseErr != nil {
			log.Errorln(logTag, ": ", parseErr.Error())
			util.WriteBackError(w, parseErr.Error(), http.StatusInternalServerError)
			return
		}

		independentReqBody, independentErr := FromIndependentRequestContext(req.Context())
		if independentErr != nil {
			log.Errorln(logTag, ": ", err)
			util.WriteBackError(w, "Can't read independent requests built", http.StatusBadRequest)
			return
		}

		// Add the independent requests to the validate body to return
		for _, independentReq := range *independentReqBody {
			validateMapToShow = append(validateMapToShow, independentReq)
		}

		// Iterate over all the requests and remove sensitive headers if any.
		BLACKLISTED_HEADERS := []string{
			"Authorization",
		}

		for validateIndex, validateMap := range validateMapToShow {
			endpointAsMap := validateMap["endpoint"].(map[string]interface{})

			headersAsMap := endpointAsMap["headers"].(map[string]interface{})

			for _, blacklistedHeader := range BLACKLISTED_HEADERS {
				delete(headersAsMap, blacklistedHeader)
				delete(headersAsMap, strings.ToLower(blacklistedHeader))
			}

			validateMapToShow[validateIndex] = validateMap
		}

		// Marshal the validate response
		marshalledResponse, marshalErr := json.Marshal(validateMapToShow)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling response, ", marshalErr)
			log.Warnln(logTag, ": ", errMsg)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(string(marshalledResponse)))
	}
}

// ParseMsearchToValidate will parse the passed msearch body to its validate
// equivalent
func ParseMsearchToValidate(reqBodySplitted []string, requestURL string, methodUsed string, headersPassed map[string]interface{}) ([]map[string]interface{}, error) {
	validateMapToShow := make([]map[string]interface{}, 0)

	// The first item in the array will be the map that will contain the
	// preference.
	// Second object will be the body for that request.
	for reqIndex, reqPref := range reqBodySplitted {
		// We will skip all odd values since those will be worked
		// on during even values.
		if reqIndex%2 != 0 {
			continue
		}

		requestBody := reqBodySplitted[reqIndex+1]

		// Unmarshal into map
		prefAsMap := make(map[string]interface{})
		bodyAsMap := make(map[string]interface{})

		prefUnmarshalErr := json.Unmarshal([]byte(reqPref), &prefAsMap)
		if prefUnmarshalErr != nil {
			errMsg := fmt.Sprintf("error while unmarshalling preferences at index `%d` with err: %v", reqIndex, prefUnmarshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return validateMapToShow, fmt.Errorf(errMsg)
		}

		reqUnmarshalErr := json.Unmarshal([]byte(requestBody), &bodyAsMap)
		if reqUnmarshalErr != nil {
			errMsg := fmt.Sprintf("error while unmarshalling request at index `%d` with err: %v", reqIndex+1, reqUnmarshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return validateMapToShow, fmt.Errorf(errMsg)
		}

		// Extract the preference string
		preferenceAsString := prefAsMap["preference"].(string)
		requestID := extractIDFromPreference(preferenceAsString)

		validateMapToShow = append(validateMapToShow, map[string]interface{}{
			"id": requestID,
			"endpoint": map[string]interface{}{
				"url":     util.CleanPasswordFromURL(requestURL),
				"method":  methodUsed,
				"headers": headersPassed,
				"body":    bodyAsMap,
			},
		})
	}

	return validateMapToShow, nil
}

// FilterAggKeys will filter the passed keys from the byte object.
func FilterAggKeys(allAggs []byte, keysToKeep []string) []byte {
	var filteredJSON []byte

	jsonparser.ObjectEach(allAggs, func(key, value []byte, dataType jsonparser.ValueType, offset int) error {
		for _, keyToKeep := range keysToKeep {
			if string(key) == keyToKeep {
				filteredJSON = append(filteredJSON, append([]byte(fmt.Sprintf(`"%s"`, string(key))), ':')...)
				filteredJSON = append(filteredJSON, value...)
				filteredJSON = append(filteredJSON, ',')
				break
			}
		}
		return nil
	})

	// Remove the trailing comma if present
	if len(filteredJSON) > 0 && filteredJSON[len(filteredJSON)-1] == ',' {
		filteredJSON = filteredJSON[:len(filteredJSON)-1]
	}

	// Enclose the filtered JSON in curly braces to form a valid JSON object
	filteredJSON = append([]byte{'{'}, append(filteredJSON, '}')...)

	return filteredJSON
}

// UnOptimizeResponse will convert the ES response to the un-optimized
// version based on the details provided.
func UnOptimizeResponse(response []byte, queryIdToMSearchDetails []MSearchDetails) ([]byte, error) {
	unOptimizedResponses := make([][]byte, 0)

	optimizedResponses, valueType, _, getErr := jsonparser.Get(response, "responses")
	if getErr != nil || valueType == jsonparser.NotExist || optimizedResponses == nil {
		return response, nil
	}

	if len(queryIdToMSearchDetails) == 0 {
		return response, nil
	}

	for _, mSearchDetails := range queryIdToMSearchDetails {
		queryReferenced, queryValueType, _, getErr := jsonparser.Get(response, "responses", fmt.Sprintf("[%d]", mSearchDetails.index))
		if getErr != nil {
			return nil, fmt.Errorf("error while fetching referenced query: %s", getErr.Error())
		}

		if queryValueType == jsonparser.NotExist {
			return nil, fmt.Errorf("invalid index passed for reference query")
		}

		// Extract the aggs value from the query.
		//
		// If it is not present, we can ignore the aggs from the query
		// and continue.
		aggsFromQuery, aggsValueType, _, aggsGetErr := jsonparser.Get(queryReferenced, "aggregations")

		if aggsValueType == jsonparser.NotExist {
			// Aggs doesn't exist, we can continue.
			unOptimizedResponses = append(unOptimizedResponses, queryReferenced)
			continue
		}

		if aggsGetErr != nil {
			return nil, fmt.Errorf("error while fetching aggs from referenced query: %s", aggsGetErr.Error())
		}

		// Aggregations are present, we will need to filter out the ones
		// that are part of the current query.
		filteredAggKeys := FilterAggKeys(aggsFromQuery, mSearchDetails.aggregationNames)

		// If there are no aggs to be set, just remove the keys
		if string(filteredAggKeys) == "{}" {
			queryWithoutAggs := jsonparser.Delete(queryReferenced, "aggregations")
			unOptimizedResponses = append(unOptimizedResponses, queryWithoutAggs)
			continue
		}

		queryWithAggs, setErr := jsonparser.Set(queryReferenced, filteredAggKeys, "aggregations")
		if setErr != nil {
			return nil, fmt.Errorf("error while setting updated aggs key: %s", setErr.Error())
		}

		unOptimizedResponses = append(unOptimizedResponses, queryWithAggs)
	}

	log.Debug(logTag, ": queries fetched count: ", len(unOptimizedResponses))

	// Reset the older response to an array with size of the un-optimized
	// responses array.
	repetitions := ""
	if len(unOptimizedResponses) > 0 {
		repetitions = strings.Repeat("{},", len(unOptimizedResponses)-1)
	}

	emptyArrStr := []byte("[" + repetitions + "{}" + "]")
	olderResponsesReset, resetErr := jsonparser.Set(response, emptyArrStr, "responses")
	if resetErr != nil {
		return nil, fmt.Errorf("error while setting responses key as empty array: %s", resetErr.Error())
	}

	// Create a responses array and put all the un-optimized responses
	// in there.
	updatedResponse := olderResponsesReset

	log.Debug(logTag, ": updated response: ", string(updatedResponse))

	for responseIndex, response := range unOptimizedResponses {
		var updateErr error
		updatedResponse, updateErr = jsonparser.Set(updatedResponse, response, "responses", fmt.Sprintf("[%d]", responseIndex))
		if updateErr != nil {
			return nil, fmt.Errorf("error while updating responses while appending: %s", updateErr.Error())
		}
	}

	log.Debug(logTag, ": updated response: ", string(updatedResponse))

	return updatedResponse, nil
}

func TransformESResponse(response []byte, rsAPIRequest *RSQuery, queryIdToMsearchDetails []MSearchDetails) ([]byte, error) {
	queryIds := GetQueryIds(*rsAPIRequest)

	rsResponse := []byte(`{}`)

	if response == nil {
		response = []byte(`{ "took": 0 }`)
	}

	// Before starting any parsing of the response body received from ES,
	// we will update the body to make it be like the un-combined/un-duplicated
	// version so that all the parsing works perfectly fine.
	var unOptimizeErr error
	response, unOptimizeErr = UnOptimizeResponse(response, queryIdToMsearchDetails)
	if unOptimizeErr != nil {
		return nil, fmt.Errorf("error while un-optimizing the response from ES: %s", unOptimizeErr.Error())
	}

	mockedRSResponse, _ := json.Marshal(ES_MOCKED_RESPONSE)
	for _, query := range rsAPIRequest.Query {
		if query.Type == Suggestion {
			// mock empty response for suggestions when index/endpoint suggestions are disabled
			//
			// Suggestions are disabled if both index suggestions and endpoint suggestions
			// are explicitly disabled.
			isSuggestionDisabled := !((query.EnableIndexSuggestions == nil || *query.EnableIndexSuggestions) ||
				(query.Endpoint != nil && query.EnableEndpointSuggestions != nil && *query.EnableEndpointSuggestions))

			if isSuggestionDisabled {
				rsResponseMocked, err := jsonparser.Set(rsResponse, mockedRSResponse, *query.ID)
				rsResponse = rsResponseMocked
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil, errors.New("error updating response :" + err.Error())
				}
			}
		}
	}

	took, valueType1, _, err := jsonparser.Get(response, "took")
	// ignore not exist error
	if err != nil && valueType1 != jsonparser.NotExist {
		log.Errorln(logTag, ":", err)
		return nil, errors.New("can't parse took key from response")
	}
	if string(took) != "" {
		// Set the `settings` key to response
		rsResponseWithTook, err := jsonparser.Set(rsResponse, []byte(fmt.Sprintf(`{ "took": %s }`, string(took))), "settings")
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, errors.New("can't add settings key to response")
		}
		// Assign updated json to actual response
		rsResponse = rsResponseWithTook
	}

	responseError, valueType2, _, err := jsonparser.Get(response, "error")
	// ignore not exist error
	if err != nil && valueType2 != jsonparser.NotExist {
		log.Errorln(logTag, ":", err)
		return nil, errors.New("can't parse error key from response")
	} else if responseError != nil {
		// Set the `error` key to response
		rsResponseWithError, err := jsonparser.Set(rsResponse, responseError, "error")
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, errors.New("can't add error key to response")
		}
		// Assign updated json to actual response
		rsResponse = rsResponseWithError
	}

	// Read `responses` value from the response body
	responses, valueType3, _, err4 := jsonparser.Get(response, "responses")
	// ignore not exist error
	if err4 != nil && valueType3 != jsonparser.NotExist {
		log.Errorln(logTag, ":", err4)
		return nil, errors.New("can't parse responses key from response")
	}

	if responses != nil {
		index := 0
		var parsingError error
		// Set `responses` by query ID
		jsonparser.ArrayEach(responses, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			if index < len(queryIds) {
				queryID := queryIds[index]
				var isSuggestionRequest bool
				var suggestions = make([]SuggestionHIT, 0)
				// parse suggestions if query is of type `suggestion`
				for _, query := range rsAPIRequest.Query {
					if *query.ID == queryID && query.Type == Suggestion {
						isSuggestionRequest = true

						// Parse the query value. If it is present, use it
						// as is. If it is not present then treat it like
						// the value being an empty string `""`.
						queryValue := query.Value
						if queryValue == nil {
							var defaultQueryValue interface{} = ""
							queryValue = &defaultQueryValue
						}

						// Index suggestions are not meant for empty query
						valueAsString, ok := (*queryValue).(string)
						if ok {
							var normalizedDataFields = []string{}
							normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
							for _, dataField := range normalizedFields {
								normalizedDataFields = append(normalizedDataFields, dataField.Field)
							}

							// Parse the valueFields passed in indexSuggestionsConfig
							var valueFields = []string{}
							if query.IndexSuggestionsConfig != nil && query.IndexSuggestionsConfig.ValueFields != nil {
								valueFields = *query.IndexSuggestionsConfig.ValueFields
							}

							suggestionsConfig := SuggestionsConfig{
								// Fields to extract suggestions
								DataFields: normalizedDataFields,
								// Query value
								Value:                       valueAsString,
								ShowDistinctSuggestions:     query.ShowDistinctSuggestions,
								EnablePredictiveSuggestions: query.EnablePredictiveSuggestions,
								MaxPredictedWords:           query.MaxPredictedWords,
								EnableSynonyms:              query.EnableSynonyms,
								ApplyStopwords:              query.ApplyStopwords,
								Stopwords:                   query.Stopwords,
								URLField:                    query.URLField,
								CategoryField:               query.CategoryField,
								HighlightField:              query.HighlightField,
								HighlightConfig:             query.HighlightConfig,
								Language:                    query.SearchLanguage,
								IndexSuggestionsConfig:      query.IndexSuggestionsConfig,
								ValueFields:                 valueFields,
							}

							var rawHits []ESDoc
							hits, dataType, _, err1 := jsonparser.Get(value, "hits", "hits")
							if dataType == jsonparser.NotExist {
								// write raw response
								rsResponseWithSearchResponse, err := jsonparser.Set(rsResponse, value, queryID)
								if err != nil {
									log.Errorln(logTag, ":", err)
									parsingError = errors.New("can't add search response to final response")
									return
								}
								rsResponse = rsResponseWithSearchResponse
								continue
							}
							if err1 != nil {
								log.Errorln(logTag, ":", err1)
								parsingError = errors.New("error while retriving hits: " + err1.Error())
								return

							}
							err := json.Unmarshal(hits, &rawHits)
							if err != nil {
								log.Errorln(logTag, ":", err)
								parsingError = errors.New("error while parsing ES response to hits: " + err.Error())
								return
							}
							// extract category suggestions
							if query.CategoryField != nil && *query.CategoryField != "" {
								categories, dataType2, _, err2 := jsonparser.Get(value, "aggregations", *query.CategoryField, "buckets")
								if err2 != nil {
									log.Errorln(logTag, ":", err2)
									parsingError = errors.New("error while retriving aggregations: " + err2.Error())
									return
								}
								if dataType2 != jsonparser.NotExist {
									var buckets []es7.AggregationBucketKeyItem
									err := json.Unmarshal(categories, &buckets)
									if err != nil {
										log.Errorln(logTag, ":", err)
										parsingError = errors.New("error while parsing ES aggregations to suggestions: " + err.Error())
										return
									}
									for _, v := range buckets {
										key, ok := v.Key.(string)
										if ok {
											count := int(v.DocCount)
											sectionId := "index"
											var sectionLabel *string
											if query.IndexSuggestionsConfig != nil {
												sectionLabel = query.IndexSuggestionsConfig.SectionLabel
											}
											suggestions = append(suggestions, SuggestionHIT{
												Label:        valueAsString + " in " + key,
												Value:        valueAsString,
												Count:        &count,
												Category:     &key,
												SectionId:    &sectionId,
												SectionLabel: sectionLabel,
											})
										}
									}
								}
							}

							// extract index suggestions
							suggestions = append(suggestions, getIndexSuggestions(suggestionsConfig, rawHits)...)
							if query.Size != nil &&
								!(query.EnableFeaturedSuggestions != nil && *query.EnableFeaturedSuggestions) && !(query.EnableFAQSuggestions != nil && *query.EnableFAQSuggestions) {
								// fit suggestions to the max requested size
								// unless when featured suggestions or FAQ suggestions are enabled
								if len(suggestions) > *query.Size {
									suggestions = suggestions[:*query.Size]
								}
							}
						}
					}
				}
				if isSuggestionRequest {
					responseInByte, err := json.Marshal(suggestions)
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("error while parsing suggestions:" + err.Error())
						return
					}
					rsResponseWithSuggestions, err := jsonparser.Set(value, responseInByte, "hits", "hits")
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("can't add suggestions to final response" + err.Error())
						return
					}
					rsResponseWithSearchResponse, err := jsonparser.Set(rsResponse, rsResponseWithSuggestions, queryID)
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("can't add search response to final response" + err.Error())
						return
					}
					// Modify total suggestions value
					rsResponseWithSearchResponse, err2 := jsonparser.Set(rsResponseWithSearchResponse, []byte(strconv.Itoa(len(suggestions))), queryID, "hits", "total", "value")
					if err2 != nil {
						log.Errorln(logTag, ":", err2)
						parsingError = errors.New("can't apply total value for hits" + err2.Error())
						return
					}
					rsResponse = rsResponseWithSearchResponse
				} else {
					rsResponseWithSearchResponse, err := jsonparser.Set(rsResponse, value, queryID)
					if err != nil {
						log.Errorln(logTag, ":", err)
						parsingError = errors.New("can't add search response to final response" + err.Error())
						return
					}
					rsResponse = rsResponseWithSearchResponse
				}
				index++
			}
		})
		if parsingError != nil {
			return nil, parsingError
		}
	}
	return rsResponse, nil
}

// HandleApiSchema will handle returning the RS API body
// schema
func (r *QueryTranslate) HandleApiSchema() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Infoln(logTag, ": returning already marshalled schema as bytes")
		util.WriteBackRaw(w, r.apiSchema, http.StatusOK)
	}
}
