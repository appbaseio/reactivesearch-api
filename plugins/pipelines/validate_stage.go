package pipelines

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// executeValidateStage will execute the validation related
// checks, build a validate response based on the body and accordingly
// stop the execution of the pipeline.
func executeValidateStage(
	stage ESPipelineStage,
	req *http.Request,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	// Check if the stage should be triggered, this will be determined
	// based on a value in the context.
	isValidateFromCtx, validateFetchErr := PipelineIsValidateFromContext(req.Context())
	if validateFetchErr != nil {
		errMsg := fmt.Sprint("error while reading to see if the validate stage should be executed: ", validateFetchErr.Error())
		log.Errorln(logTag, ": ", errMsg)
		return nil, false, &Error{
			Err: fmt.Errorf(errMsg),
		}
	}

	if isValidateFromCtx == nil || !*isValidateFromCtx {
		return nil, false, nil
	}

	// id := getStageID(stage)
	scriptContextInBytes := globalScriptContext.Get()
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}

	// NOTE: We don't support inputs for this stage so no need
	// to parse inputs or verify them either.

	method := http.MethodPost
	reqMethod, ok := scriptEnvs["method"].(string)
	if ok {
		method = reqMethod
	}

	headers := http.Header{}
	for k := range scriptContext.Request.Headers {
		if k == "Authorization" || k == "Accept" {
			continue
		}
		headers.Set(k, scriptContext.Request.Headers[k])
	}

	headersMap := make(map[string]interface{})
	for k := range headers {
		headersMap[k] = headers.Get(k)
	}

	// Read the backend that would be injected by the RS stage and based on
	// that parse the requests accordingly.
	backendAsInterface, backendPresent := scriptContext.Environments["RS_BACKEND"]
	if !backendPresent {
		errMsg := fmt.Sprint("seems like the `reactivesearchQuery` stage was not executed before this stage, cannot continue!")
		log.Warnln(logTag, ": ", errMsg)
		return nil, false, &Error{
			Err: fmt.Errorf(errMsg),
		}
	}

	backendAsString, asStrOk := backendAsInterface.(string)
	if !asStrOk {
		// Set the backend as ES
		backendAsString = "elasticsearch"
	}

	validateMap := make([]map[string]interface{}, 0)

	if backendAsString == "mongodb" {
		// The request body here will be a map where each key is the id of the request
		// and value is an array (mongodb pipeline).
		var mdbBodyAsMap = make(map[string]interface{})
		unmarshalErr := json.Unmarshal([]byte(scriptContext.Request.Body), &mdbBodyAsMap)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling mdb body into map: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		for requestId, mdbBodyEach := range mdbBodyAsMap {
			validateBodyEach := map[string]interface{}{
				"endpoint": map[string]interface{}{
					"body":    mdbBodyEach,
					"headers": headersMap,
					"method":  method,
					"url":     "",
				},
				"headers": headersMap,
				"id":      requestId,
			}

			validateMap = append(validateMap, validateBodyEach)
		}
	} else if backendAsString == "solr" {
		// Parse JSON request body
		var queries map[string]interface{}
		unmarshalErr := json.Unmarshal([]byte(scriptContext.Request.Body), &queries)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling solr body into map: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		urlValues, areValuesPresent := scriptContext.Environments["urlValues"]
		if !areValuesPresent {
			errMsg := fmt.Sprint("error while parsing urlValues, not present")
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		// Make sure that we can read from urlValues as a map

		// collection is `urlValues.index`
		// queryString is `urlValues.queryString`
		urlValuesAsInterfaceMap, asMapOk := urlValues.(map[string]interface{})
		if !asMapOk {
			errMsg := fmt.Sprint("urlValues is not a map, cannot parse index!")
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		collectionName := ""
		indexParsed, isIndexPresent := urlValuesAsInterfaceMap["index"]
		if isIndexPresent {
			collectionName = indexParsed.(string)
		}

		qs := ""
		qsParsed, isQSPresent := urlValuesAsInterfaceMap["qs"]
		if isQSPresent {
			qs = qsParsed.(string)
		}

		// Read envs.protocol and envs.host
		protocolAsStr := ""
		protocol, isPresent := scriptContext.Environments["protocol"]
		if !isPresent {
			protocolAsStr = "http"
		} else {
			protocolAsStr = protocol.(string)
		}

		hostAsStr := ""
		host, isHostPresent := scriptContext.Environments["host"]
		if !isHostPresent {
			hostAsStr = "127.0.0.1"
		} else {
			hostAsStr = host.(string)
		}

		// Construct URL

		URL := fmt.Sprintf("%s://%s/solr/%s/select", protocolAsStr, hostAsStr, collectionName)

		for key, queryEach := range queries {
			queryMap := queryEach.(map[string]interface{})

			// Handle non-endpoint queries
			isEndpoint, isEndpointPresent := queryMap["_isEndpoint"]
			if !isEndpointPresent || isEndpoint.(string) != "true" {
				// Build query string
				queryString := []string{}
				for k, v := range queryMap {
					if k != "_original" && k != "_isEndpoint" {
						queryString = append(queryString, fmt.Sprintf("%s=%s", k, url.QueryEscape(v.(string))))
					}
				}
				built := strings.Join(queryString, "&")
				finalURL := URL + "?"

				if qs != "" {
					finalURL += qs + "&"
				}

				finalURL += built

				validateMap = append(validateMap, map[string]interface{}{
					"id": key,
					"endpoint": map[string]interface{}{
						"url":     finalURL,
						"method":  "GET",
						"headers": headersMap,
						"body":    map[string]interface{}{},
					},
				})
			} else {
				validateMap = append(validateMap, map[string]interface{}{
					"id":       queryMap["id"],
					"endpoint": queryMap["endpoint"],
				})
			}
		}
	} else {
		// Request body is nd-json so we need to convert it into an
		// array of strings by splitting on \n
		reqBodySplitted := strings.Split(string(scriptContext.Request.Body), "\n")

		// Remove the last item since it's empty
		if len(reqBodySplitted) > 0 {
			reqBodySplitted = reqBodySplitted[:len(reqBodySplitted)-1]
		}

		var parseErr error
		validateMap, parseErr = querytranslate.ParseMsearchToValidate(reqBodySplitted, util.GetESURL(), method, headersMap)
		if parseErr != nil {
			errMsg := fmt.Sprint("error while parsing request into validate equivalent: ", parseErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}
	}

	independentRequestsAsInterface, isPresent := scriptContext.Environments["INDEPENDENT_REQUESTS"]
	if isPresent {
		independentRequestsAsString, asStrOk := independentRequestsAsInterface.(string)
		if !asStrOk {
			errMsg := fmt.Sprint("could not parse independent requests to string!")
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		independentRequestsAsMap := make([]map[string]interface{}, 0)
		independentUnmarshalErr := json.Unmarshal([]byte(independentRequestsAsString), &independentRequestsAsMap)
		if independentUnmarshalErr != nil {
			errMsg := fmt.Sprint("error unmarshalling independent requests to map: ", independentUnmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err: fmt.Errorf(errMsg),
			}
		}

		// Add the independent requests to the validate body to return
		for _, independentReq := range independentRequestsAsMap {
			validateMap = append(validateMap, independentReq)
		}
	}

	// Iterate over all the requests and remove sensitive headers if any.
	BLACKLISTED_HEADERS := []string{
		"Authorization",
	}

	for validateIndex, validateMapEach := range validateMap {
		endpointAsMap := validateMapEach["endpoint"].(map[string]interface{})

		headersAsInterface, areHeadersPresent := endpointAsMap["headers"]
		if !areHeadersPresent {
			continue
		}

		headersAsMap, isOkay := headersAsInterface.(map[string]interface{})
		if !isOkay {
			continue
		}

		for _, blacklistedHeader := range BLACKLISTED_HEADERS {
			delete(headersAsMap, blacklistedHeader)
			delete(headersAsMap, strings.ToLower(blacklistedHeader))
		}

		validateMapEach["headers"] = headersAsMap
		validateMap[validateIndex] = validateMapEach
	}

	// Marshal the validate response
	marshalledResponse, marshalErr := json.Marshal(validateMap)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling response, ", marshalErr)
		log.Warnln(logTag, ": ", errMsg)
		return nil, false, &Error{
			Err: fmt.Errorf(errMsg),
		}
	}

	var output interface{}
	// set response code
	scriptContext.Response.Code = http.StatusOK
	scriptContext.Response.Body = string(marshalledResponse)
	scriptContext.Response.Headers["X-Origin"] = "reactivesearch.io"

	output = scriptContext
	contextInBytes, err := json.Marshal(output)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}

	return contextInBytes, true, nil
}
