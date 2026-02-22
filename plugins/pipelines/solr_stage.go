package pipelines

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/plugins/suggestions"
	"github.com/appbaseio/reactivesearch-api/plugins/uibuilder"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/buger/jsonparser"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
)

type SolrInput struct {
	Protocol          *string            `json:"protocol,omitempty" jsonschema:"title=Protocol" jsonschema_description:"Protocol to use for connecting. Defaults to 'http'."`
	Host              *string            `json:"host,omitempty" jsonschema:"title=Host" jsonschema_description:"Host name and port for Fusion. For example: 'localhost:6764'"`
	App               *string            `json:"app,omitempty" jsonschema:"title=App" jsonschema_description:"App to search the query in. Example: 'appbase'"`
	Profile           *string            `json:"profile,omitempty" jsonschema:"title=Profile" jsonschema_description:"Profile in Fusion to search the query in. Example: 'appbase'"`
	SuggestionProfile *string            `json:"suggestionProfile,omitempty" jsonschema:"title=Suggestion Profile" jsonschema_description:"Profile in Fusion to send the request to for suggestion type of query. Example: 'popular-searches'"`
	Collection        *string            `json:"collection,omitempty" jsonschema:"title=Collection" jsonschema_description:"Collection in Solr to run the query against. Example: 'appbase'"`
	Credentials       *string            `json:"credentials,omitempty" jsonschema:"title=Credentials" jsonschema_description:"Credentials to access the fusion host. Example: 'username:password'"`
	Headers           *map[string]string `json:"headers,omitempty" jsonschema:"title=Headers" jsonschema_description:"Headers to be passed in the Solr query request."`
	URI               *string            `json:"uri,omitempty" jsonschema:"title=URI" jsonschema_description:"URI to connect to the fusion instance. If passed, host, app, profile and credentials will be ignored and not required. Example: 'http://username:password@localhost:6764/api/apps/appbase/query/appbase'"`
	Query             *string            `json:"query,omitempty" jsonschema:"title=Query" jsonschema_description:"Query to run on Solr. If passed, this query will be prioritized over the query built from ReactiveSearch stage. Example: 'q=*:*&qf=some_field'"`
	QueryString       *map[string]string `json:"queryString,omitempty" jsonschema:"title=Query String" jsonschema_description:"Query string to pass in the final query along with the built query. Should be an object with key value pairs. Example: 'extraParams=debug'"`
}

// GetSolrInputSchema will return the schema for Solr
func GetSolrInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&SolrInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeSolrStage will execute the solr request and
// return the response after parsing it to
// ReactiveSearch equivalent.
func executeSolrStage(
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
	if parsedInputs == nil {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Inputs are missing for stage id: " + *id),
			Code: http.StatusBadRequest,
		}
	}

	// Parse the inputs to SolrInput
	var inputs SolrInput
	inputParseErr := json.Unmarshal([]byte(*parsedInputs), &inputs)
	if inputParseErr != nil {
		log.Warnln(logTag, ": error while parsing inputs to Solr input, ", inputParseErr)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Error while trying to parse inputs as Solr ones: %s", inputParseErr),
			Code: http.StatusBadRequest,
		}
	}

	// If both host and URI are not passed, we can't work.
	if (inputs.URI == nil || *inputs.URI == "") && (inputs.Host == nil || *inputs.Host == "") {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("One of `uri` or `host` is required."),
			Code: http.StatusBadRequest,
		}
	}

	// Either of URI or collection or `app` and `profile` should be present
	if (inputs.URI == nil || *inputs.URI == "") && (inputs.App == nil || *inputs.App == "" || inputs.Profile == nil || *inputs.Profile == "") && (inputs.Collection == nil || *inputs.Collection == "") {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("One of `uri`, `app and profile` or `collection` is required."),
			Code: http.StatusBadRequest,
		}
	}

	if inputs.Protocol == nil {
		defaultProtocol := "http"
		inputs.Protocol = &defaultProtocol
	}

	// default to passed profile if SuggestionProfile is not
	// passed.
	//
	// NOTE: Assumption here is that `profile` will always be
	// passed else an error will be thrown.
	if inputs.SuggestionProfile == nil {
		inputs.SuggestionProfile = inputs.Profile
	}

	if inputs.Headers == nil {
		inputs.Headers = &map[string]string{}
	}

	// Set the credentials if it is passed
	if inputs.Credentials != nil {
		(*inputs.Headers)["Authorization"] = fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(*inputs.Credentials)))
	}

	FusionURITemplate := "%s://%s/api/apps/%s/query/%s"
	SolrURITemplate := "%s://%s/solr/%s/select"
	builtSuggestionURI := ""
	builtURI := ""
	if inputs.URI == nil {

		// We will reach this condition only if one of collection or (app and profile) are present
		// so we can use the collection building URL on an else below.
		if inputs.App != nil && inputs.Profile != nil {
			builtURI = fmt.Sprintf(FusionURITemplate, *inputs.Protocol, *inputs.Host, *inputs.App, *inputs.Profile)
			builtSuggestionURI = fmt.Sprintf(FusionURITemplate, *inputs.Protocol, *inputs.Host, *inputs.App, *inputs.SuggestionProfile)
		} else {
			builtURI = fmt.Sprintf(SolrURITemplate, *inputs.Protocol, *inputs.Host, *inputs.Collection)
		}

		inputs.URI = &builtURI
	}

	// Parse the queryString input. If not passed then read the urlValues map
	// from context and set it as the queryString map
	if inputs.QueryString == nil {
		urlValues, ok := scriptContext.Environments["urlValues"]
		if ok {
			urlValuesAsMap, asMapOk := urlValues.(map[string]interface{})
			queryStringMap := make(map[string]string)
			if asMapOk {
				for key, valueAsInterface := range urlValuesAsMap {
					valueAsStr, asStrOk := valueAsInterface.(string)
					if asStrOk {
						queryStringMap[key] = valueAsStr
					}
				}
			}

			// Set the queryStringMap
			inputs.QueryString = &queryStringMap
		}
	}

	queriesToRun := make(map[string]map[string]interface{})

	parseResponseToReactivesearch := false

	if inputs.Query != nil {
		parsedQuery, parseErr := url.ParseQuery(*inputs.Query)
		if parseErr != nil {
			errMsg := fmt.Sprint("query parsing failed, invalid query: ", parseErr)
			log.Warnln(logTag, ": ", errMsg)
			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusBadRequest,
			}
		}

		// url.Value is of type map[string][]string but we want it
		// to be map[string]string
		parsedQueryMap := make(map[string]interface{})
		for key, value := range parsedQuery {
			parsedQueryMap[key] = strings.Join(value, " ")
		}

		queriesToRun["solrQuery"] = parsedQueryMap
	} else {
		// Parse the query from the request body
		//
		// TODO: Handle the case of `reactivesearchQuery` run in `async`
		// mode, we won't be able to extract the body directly in that
		// case.
		unmarshalErr := json.Unmarshal([]byte(scriptContext.Request.Body), &queriesToRun)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling request body from RS stage, ", unmarshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		parseResponseToReactivesearch = true
	}

	var currentBest *querytranslate.Query

	// Determine which query will be used to return the `fusionQueryID`.
	//
	// This logic will give the priority in the following order:
	// - search
	// - geo
	// - suggestion
	//
	// If search type is present more than once, the first one will be considered.
	// This is why if search type of query is found, it will be returned right away.
	//
	// Queries that are to be executed (i:e `execute` is `true`) will only be considered
	// , others will be ignored.
	for _, queryToRun := range queriesToRun {
		originalQuery, isKeyPresent := queryToRun["_original"]
		originalRSQuery := querytranslate.Query{}

		if isKeyPresent {
			originalQueryString := originalQuery.(string)

			// Parse the RSQuery
			unmarshallErr := json.Unmarshal([]byte(originalQueryString), &originalRSQuery)
			if unmarshallErr != nil {
				errMsg := fmt.Sprint("error while unmarshalling original body into RS body, ", unmarshallErr)
				log.Errorln(logTag, ": ", errMsg)

				return scriptContextInBytes, false, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusInternalServerError,
				}
			}
		}

		// Continue only if the query is supposed to execute.
		if originalRSQuery.Execute != nil && !*originalRSQuery.Execute {
			continue
		}

		if currentBest == nil {
			// Set the current value. This case is probably during the
			// first element in the array.
			currentBest = &originalRSQuery
		} else {
			// Check if the current one is better than the current best.
			if originalRSQuery.Type < currentBest.Type {
				currentBest = &originalRSQuery
			}
		}

		// If type is search than exit the loop since we want the first
		// search type of query.
		if currentBest.Type == querytranslate.Search {
			break
		}
	}

	// Start the featured_suggestions execution if type is passed
	var featuredSuggestionsWg sync.WaitGroup
	featuredSuggestionsOut := make(chan suggestions.SuggestionOutput)

	if parseResponseToReactivesearch {
		requestQuery := rsAPIRequest.Get()
		// fetch popular and recent suggestions
		for _, query := range requestQuery.Query {
			if query.Type == querytranslate.Suggestion {
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

	// Set the query ID for fusion query ID extraction
	shouldExtractFusionQueryId := currentBest != nil && currentBest.ID != nil
	queryIdForFusionQuery := ""
	if shouldExtractFusionQueryId {
		queryIdForFusionQuery = *currentBest.ID
	}

	combinedOutput := make(map[string]interface{})
	var maxTook float64 = 0
	settingsMap := make(map[string]interface{})
	for queryId, queryToRun := range queriesToRun {
		// Determine whether this is a Solr query or an endpoint query
		// based on the `_isEndpoint` flag.
		isEndpointValue, isEndpointPresent := queryToRun["_isEndpoint"]

		// If endpoint is present and true than skip it since we will execute
		// it in a next stage.
		if isEndpointPresent && isEndpointValue == "true" {
			continue
		}

		// Remove the original query string
		//
		// NOTE: Handle if the _original field is not passed, i:e if query
		// is passed directly.
		originalQuery, isKeyPresent := queryToRun["_original"]
		originalRSQuery := querytranslate.Query{}

		queryAsStrMap := make(map[string]string)
		for key, value := range queryToRun {
			valueAsStr, asStrOk := value.(string)
			if !asStrOk {
				errMsg := "error while converting map of interface to map of string"
				log.Warnln(logTag, ": ", errMsg)
				return scriptContextInBytes, false, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusInternalServerError,
				}
			}

			queryAsStrMap[key] = valueAsStr
		}

		if isKeyPresent {
			originalQueryString := originalQuery.(string)

			// Delete the key
			delete(queryToRun, "_original")

			// Parse the RSQuery
			unmarshallErr := json.Unmarshal([]byte(originalQueryString), &originalRSQuery)
			if unmarshallErr != nil {
				errMsg := fmt.Sprint("error while unmarshalling original body into RS body, ", unmarshallErr)
				log.Errorln(logTag, ": ", errMsg)

				return scriptContextInBytes, false, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusInternalServerError,
				}
			}
		}

		// Check if `_index` is present in the query, and if so then update the URL
		// based on that
		indexValue, indexOk := queryAsStrMap["_index"]
		if indexValue == "" {
			indexOk = false
		}
		delete(queryAsStrMap, "_index")

		if indexOk {
			if inputs.App != nil && inputs.Profile != nil {
				inputs.Profile = &indexValue
				builtURI = fmt.Sprintf(FusionURITemplate, *inputs.Protocol, *inputs.Host, *inputs.App, *inputs.Profile)
				builtSuggestionURI = fmt.Sprintf(FusionURITemplate, *inputs.Protocol, *inputs.Host, *inputs.App, *inputs.SuggestionProfile)
			} else {
				inputs.Collection = &indexValue
				builtURI = fmt.Sprintf(SolrURITemplate, *inputs.Protocol, *inputs.Host, *inputs.Collection)
			}

			inputs.URI = &builtURI
		}

		// Before running the query, if the `q` field needs to be resolved, it will have to
		// be done before the final query.
		qValue, qOk := queryAsStrMap["q"]
		if qOk && strings.Contains(qValue, "__min__") || strings.Contains(qValue, "__max__") {
			// Resolve the min and max values in the qValue by fetching them from the stats field.
			//
			// We will use the first field present in qf
			// NOTE: No need to check for qf presence because that will be checked during conversion
			qfValue := queryAsStrMap["qf"]
			dfToUse := strings.Split(qfValue, " ")[0]

			// Get the min and max values now
			minValue, maxValue, minMaxErr := getMinMaxValue(dfToUse, *inputs.URI, inputs.Headers)
			if minMaxErr != nil {
				return scriptContextInBytes, false, &Error{
					Err:  fmt.Errorf("error while getting min max values for query: %s", minMaxErr),
					Code: http.StatusInternalServerError,
				}
			}

			qValue = strings.Replace(qValue, "__min__", fmt.Sprintf("%d", minValue), -1)
			qValue = strings.Replace(qValue, "__max__", fmt.Sprintf("%d", maxValue), -1)

			// Might need to update the interval if not passed already
			intervalInUse, intervalPresent := queryAsStrMap["facet.range.gap"]
			if intervalPresent {
				intervalAsInt, notOk := strconv.Atoi(intervalInUse)
				if notOk == nil && intervalAsInt == -1 {
					queryAsStrMap["facet.range.gap"] = fmt.Sprintf("%d", int((maxValue-minValue)/100))
				}
			}

			// Resolve the range.start and end value if they are __min__ and __max__
			facetRangeStart, startOk := queryAsStrMap["facet.range.start"]
			if startOk && facetRangeStart == "__min__" {
				queryAsStrMap["facet.range.start"] = fmt.Sprintf("%d", minValue)
			}
			facetRangeEnd, endOk := queryAsStrMap["facet.range.end"]
			if endOk && facetRangeEnd == "__max__" {
				queryAsStrMap["facet.range.end"] = fmt.Sprintf("%d", maxValue)
			}

			queryAsStrMap["q"] = qValue
		}

		// If type of query is suggestion, we need to build the URL in a different
		// way where the profile is the suggestionProfile and not the profile passed.
		//
		// We know that suggestionProfile needs to be used only when `app` and `profile`
		// inputs are passed.
		if originalRSQuery.Type == querytranslate.Suggestion && builtSuggestionURI != "" {
			inputs.URI = &builtSuggestionURI
		}

		// We need to run this and return
		responseInBytes, response, queryErr := runSolrQuery(&queryAsStrMap, inputs.URI, inputs.Headers, inputs.QueryString)

		// Set the builtURI back as the inputs.URI since we want that to be default.
		inputs.URI = &builtURI

		if queryErr != nil {
			errMsg := fmt.Sprint("error while running solr query, ", queryErr.Err.Error())
			log.Warnln(logTag, ": ", errMsg)

			return scriptContextInBytes, false, queryErr
		}

		// Unmarshal the response to a map
		responseAsMap := make(map[string]interface{})
		unmarshalErr := json.Unmarshal(responseInBytes, &responseAsMap)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling response to map: ", unmarshalErr)
			log.Warnln(logTag, ": ", errMsg)

			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		// Parse only if the status code is OK, else don't
		log.Debugln(logTag, fmt.Sprintf(": recieved status code: %d for ID: %s", response.StatusCode, queryId))

		fusionQueryIdExtraction := shouldExtractFusionQueryId && *originalRSQuery.ID == queryIdForFusionQuery

		if response.StatusCode == http.StatusOK && parseResponseToReactivesearch {
			// Parse the response
			translatedResponse, fusionQueryIdSettings, translateErr := TranslateToRS(responseAsMap, queryAsStrMap, originalRSQuery, fusionQueryIdExtraction)
			if translateErr != nil {
				errMsg := fmt.Sprintf("error while parsing Solr response to RS equivalent: %s", translateErr.Err.Error())
				log.Warnln(logTag, ": ", errMsg)

				return scriptContextInBytes, false, translateErr
			}

			// Try to parse the fusionQueryIdSettings if it was supposed to be
			// extracted
			if fusionQueryIdExtraction && fusionQueryIdSettings != nil {
				settingsMap = map[string]interface{}{
					"fusionQueryId": fusionQueryIdSettings["fusionQueryId"],
					"fusionContext": fusionQueryIdSettings["fusionContext"],
				}
			}

			// Check the took and see if it is larger than the older took
			responseTook := translatedResponse["took"].(float64)

			if responseTook > maxTook {
				maxTook = responseTook
			}

			responseAsMap = translatedResponse
		}

		combinedOutput[queryId] = responseAsMap
	}

	// Set the `took` for the settings
	settingsMap["took"] = maxTook

	// Set the settings in the combinedOutput
	combinedOutput["settings"] = settingsMap

	hostAsString, hostOk := scriptContext.Environments["origin"].(string)
	if !hostOk {
		hostAsString = ""
	}
	isTlsAsBool, boolOk := scriptContext.Environments["isTLS"].(bool)
	if !boolOk {
		isTlsAsBool = false
	}

	// Handle execution of independent queries
	for queryId, queryToRun := range queriesToRun {
		// Determine whether this is a Solr query or an endpoint query
		// based on the `_isEndpoint` flag.
		isEndpointValue, isEndpointPresent := queryToRun["_isEndpoint"]

		// If endpoint is not present or false than skip it
		if !isEndpointPresent || isEndpointValue != "true" {
			continue
		}

		responseInBytes, _, queryErr := querytranslate.ExecuteIndependentQuery(queryToRun, hostAsString, isTlsAsBool, scriptContext.Request.Headers)
		if queryErr != nil {
			errMsg := fmt.Sprintf("error while execution independent query with ID `%s` and error: %v", queryId, queryErr)
			log.Warnln(logTag, ": ", errMsg)
			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusBadRequest,
			}
		}

		// Unmarshal the response to a map
		responseAsMap := make(map[string]interface{})
		unmarshalErr := json.Unmarshal(responseInBytes, &responseAsMap)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling response to map: ", unmarshalErr)
			log.Warnln(logTag, ": ", errMsg)

			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		combinedOutput[queryId] = responseAsMap
	}

	var outputAsJSON = make([]byte, 0)
	var outputMarshalErr error

	// Return the raw response if inputs.query was passed since
	// only then parsing is disabled.
	if !parseResponseToReactivesearch {
		// Only one query was run so we need to return the raw response
		// instead of converting it to RS.
		outputAsJSON, outputMarshalErr = json.Marshal(combinedOutput["solrQuery"])
		if outputMarshalErr != nil {
			errMsg := fmt.Sprintf("error while marshalling solr output map, %s", outputMarshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

	} else {
		// Make combinedOutput a JSON
		outputAsJSON, outputMarshalErr = json.Marshal(combinedOutput)
		if outputMarshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling combined output, ", outputMarshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		var recentSuggestionsMap = make(map[string]suggestions.SuggestionOutput)
		var popularSuggestionsMap = make(map[string]suggestions.SuggestionOutput)

		// Make sure the suggestions are extracted and injected as well
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

		rsQuery := rsAPIRequest.Get()
		// apply suggestions
		responseWithSuggestions, err2 := suggestions.ApplySuggestions(*rsQuery, outputAsJSON, recentSuggestionsMap, popularSuggestionsMap, featuredSuggestionsMap, nil, nil)
		if err2 != nil {
			log.Errorln(logTag, ":", err2.Error)
			return nil, false, &Error{
				Err:  err2.Error,
				Code: err2.Code,
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
	}

	var output interface{}

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

		// Add request values
		scriptContext.Request.URL = *inputs.URI
		scriptContext.Request.Headers = *inputs.Headers
		scriptContext.Request.Method = http.MethodGet

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

// ConvertFunc will convert the passed value into a
// string to be passed in Solr
type ConvertFunc func(*querytranslate.Query, *[]querytranslate.Query) (string, string, *Error)

// RSToSolr contains a map of functions that will
// convert the passed RS Query value to it's equivalent
// Solr query.
var RSToSolr = map[string]ConvertFunc{
	"dataField":        convertDataField(),
	"value":            convertValue(),
	"queryFormat":      convertQueryFormat(),
	"size":             convertSize(),
	"from":             convertFrom(),
	"includeFields":    convertIncludeFields(),
	"react":            convertReactField(),
	"sortBy":           convertSortBy(),
	"highlight":        convertHighlight(),
	"highlightConfig":  convertHighlightConfig(),
	"calendarinterval": convertCalendarInterval(),
	"deepPagination":   convertDeepPagination(),
	"includeValues":    convertIncludeValues(),
	"excludeValues":    convertExcludeValues(),

	// Following fields are not used for conversion but are
	// directly referenced in places. It is important to keep a dummy
	// converter for them in order to make sure that these fields are supported
	// for conversion.
	"id": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return *query.ID, "", nil
	},
	"index": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return *query.ID, "", nil
	},
	"execute": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "execute", strconv.FormatBool(*query.Execute), nil
	},
	"type": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return query.Type.String(), "", nil
	},
	"showDistinctSuggestions": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return fmt.Sprint(*query.ShowDistinctSuggestions), "", nil
	},
	"enablePredictiveSuggestions": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return fmt.Sprint(*query.EnablePredictiveSuggestions), "", nil
	},
	"maxPredictedWords": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return fmt.Sprint(*query.MaxPredictedWords), "", nil
	},
	"enableSynonyms": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return fmt.Sprint(*query.EnableSynonyms), "", nil
	},
	"applyStopwords": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return fmt.Sprint(*query.ApplyStopwords), "", nil
	},
	"customStopwords": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return strings.Join(*query.Stopwords, ", "), "", nil
	},
	"urlField": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return *query.URLField, "", nil
	},
	"categoryField": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return *query.CategoryField, "", nil
	},
	"highlightField": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return strings.Join(query.HighlightField, ", "), "", nil
	},
	"searchLanguage": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return *query.SearchLanguage, "", nil
	},
	"indexSuggestionsConfig": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"enableIndexSuggestions": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"enablePopularSuggestions": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"enableFeaturedSuggestions": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"enableRecentSuggestions": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"enableEndpointSuggestions": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"showMissing": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"defaultQuery": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"customQuery": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"includeNullValues": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"popularSuggestionsConfig": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"recentSuggestionsConfig": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"featuredSuggestionsConfig": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"aggregations": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"aggregationSize": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"interval": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"sortField": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"deepPaginationConfig": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"endpoint": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"selectAllLabel": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"searchboxId": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
	"excludeFields": func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		return "", "", nil
	},
}

// ValidateRSToSolrKey will make sure that all the
// keys present in the reactivesearch request body
// are convertible to Solr equivalent.
//
// It is important to note that this method should only
// be invoked if the backend is set to Solr.
func ValidateRSToSolrKey(rsBody *[]querytranslate.Query) *Error {
	// If there is any non empty key in the rsBody that
	// is not present in RSToSolr map then we will have
	// to throw an error to let the user know that the
	// key is not supported.

	// Parse the query to a map in order to check if keys
	// are present.
	for _, query := range *rsBody {
		marshalledQuery, marshalErr := json.Marshal(query)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error occurred while marshalling query to map to validate keys for solr conversion: ", marshalErr)
			log.Errorln(logTag, ": ", errMsg)
			return &Error{
				Err:  errors.New(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		var queryAsMap map[string]interface{}
		unmarshallErr := json.Unmarshal(marshalledQuery, &queryAsMap)

		if unmarshallErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling query to map to validate conversion of RS to Solr: ", unmarshallErr)
			log.Errorln(logTag, ": ", errMsg)
			return &Error{
				Err:  errors.New(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		for key := range queryAsMap {
			_, ok := RSToSolr[key]
			if !ok {
				// Key does not exist in RSToSolr but is passed
				// We cannot allow this, so just raise an error.
				errMsg := fmt.Sprintf("%s: key is not allowed since it is not supported by Solr", key)
				log.Warnln(logTag, ": ", errMsg)
				return &Error{
					Err:  errors.New(errMsg),
					Code: http.StatusBadRequest,
				}
			}
		}

	}

	return nil
}

// runSolrQuery will run the passed solr query using the URI
// and credentials passed.
func runSolrQuery(query *map[string]string, uri *string, headers *map[string]string, queryString *map[string]string) ([]byte, *http.Response, *Error) {
	request, reqCreateErr := http.NewRequest(http.MethodGet, *uri, nil)
	if reqCreateErr != nil {
		errMsg := fmt.Sprint("error while creating the request to send Solr query, ", reqCreateErr)
		log.Errorln(logTag, ": ", errMsg)

		return nil, nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// Iterate the queryString values if present and
	// accordingly add them to the final query.
	//
	// Create a new request body just to generate the final
	// QS with the queryString values and then use that
	// string after the normal query is built.
	tempReq, _ := http.NewRequest(http.MethodGet, "https://reactivesearch.io", nil)
	tempQ := tempReq.URL.Query()
	if queryString != nil {
		for key, value := range *queryString {
			tempQ.Add(key, value)
		}
	}

	// Add the query fields
	q := request.URL.Query()
	for key, value := range *query {
		if key == "_original" || key == "_index" {
			continue
		}

		unescapedValue, unescapeErr := url.PathUnescape(value)
		if unescapeErr != nil {
			errMsg := fmt.Sprint("error while unescaping value for key: ", key)
			log.Errorln(errMsg)

			return nil, nil, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		splittedValues := strings.Split(unescapedValue, fmt.Sprint("&", key, "="))

		for _, value := range splittedValues {
			q.Add(key, value)
		}
	}

	encodedQueryStr := q.Encode()
	tempQueryStr := tempQ.Encode()

	// If the encoded string is empty, don't add the &
	if tempQueryStr != "" {
		if encodedQueryStr == "" {
			encodedQueryStr = tempQueryStr
		} else {
			encodedQueryStr = encodedQueryStr + "&" + tempQueryStr
		}
	}

	request.URL.RawQuery = encodedQueryStr

	// Set the headers in the request
	for key, value := range *headers {
		request.Header.Set(key, value)
	}

	log.Debug(logTag, ":Solr URL hit: ", request.URL.String())

	response, reqErr := util.HTTPClient().Do(request)
	if reqErr != nil {
		errMsg := fmt.Sprint("error while sending request to Solr, ", reqErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// Read the body.
	responseInBytes, readErr := ioutil.ReadAll(response.Body)
	if readErr != nil {
		errMsg := fmt.Sprint("error while reading the response body from Solr, ", readErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, response, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	return responseInBytes, response, nil
}

// TranslateToRS will translate the Solr response to its
// ReactiveSearch equivalent.
func TranslateToRS(solrResponse map[string]interface{}, queryRun map[string]string, rsQuery querytranslate.Query, shouldExtractFusionQueryID bool) (map[string]interface{}, map[string]interface{}, *Error) {
	responseAsMap := solrResponse
	RSResponseEachMap := make(map[string]interface{})
	hitsMap := make(map[string]interface{})
	settingsToReturn := make(map[string]interface{})

	// Extract the time took
	responseHeader, responseHeaderOk := responseAsMap["responseHeader"].(map[string]interface{})
	if !responseHeaderOk {
		return nil, nil, &Error{
			Err:  fmt.Errorf("Couldn't convert responseHeader to read time"),
			Code: http.StatusInternalServerError,
		}
	}

	// totalTime is present in case of Fusion but for Solr `QTime` is present.
	totalTime, isTotalTimePresent := responseHeader["totalTime"]
	if !isTotalTimePresent {
		totalTime = responseHeader["QTime"]
	}

	timeAsTook, ok := totalTime.(float64)
	if !ok {
		timeAsTook = 0
	}

	RSResponseEachMap["took"] = timeAsTook
	RSResponseEachMap["timed_out"] = false

	// Try to extract the fusionQueryId and context if it is to be extracted
	// and it is present
	if shouldExtractFusionQueryID {
		params, paramsOk := responseHeader["params"].(map[string]interface{})
		if !paramsOk {
			// Don't throw an error here since this is common in case of Solr.
			// Just silently ignore the error and don't set queryId or context
			log.Warnln(logTag, "error while parsing responseHeader.params from the Solr response")
		} else {
			// Else try to parse them.
			settingsToReturn["fusionQueryId"] = params["fusionQueryId"]
			settingsToReturn["fusionContext"] = params["context"]
		}
	}

	// Extract the nextCursorMark from the response
	//
	// This might not be always present so failing should not effect execution
	nextCursorMarkExtracted, nextCursorOk := responseAsMap["nextCursorMark"].(string)
	if !nextCursorOk {
		log.Warnln(logTag, ": ", "Couldn't extract nextCursorMark from Solr response")
	}

	RSResponseEachMap["nextCursorMark"] = nextCursorMarkExtracted

	// Extract the hits
	responseHits, hitsOk := responseAsMap["response"].(map[string]interface{})
	if !hitsOk {
		return nil, settingsToReturn, &Error{
			Err:  fmt.Errorf("Couldn't convert response.response to map"),
			Code: http.StatusInternalServerError,
		}
	}

	hitsMap["total"] = map[string]interface{}{
		"value":    responseHits["numFound"],
		"relation": "eq",
	}
	hitsMap["max_score"] = responseHits["maxScore"]

	log.Debug(logTag, ": docs", pretty.Formatter(responseHits["docs"]))

	// Extract the actual hits and calculate the max score
	actualHits, ok := responseHits["docs"].([]interface{})
	if !ok {
		return nil, settingsToReturn, &Error{
			Err:  fmt.Errorf("Error while converting actual hits to array of map"),
			Code: http.StatusInternalServerError,
		}
	}

	// Extract the `highlighting` field if it is present.
	highlightingField, isHighlightPresent := responseAsMap["highlighting"]
	highlightAsMap := make(map[string]interface{})

	if isHighlightPresent {
		highlighting, highlightOk := highlightingField.(map[string]interface{})
		if !highlightOk {
			return nil, settingsToReturn, &Error{
				Err:  fmt.Errorf("Error while parsing the `highlighting` fields to map"),
				Code: http.StatusInternalServerError,
			}
		}

		highlightAsMap = highlighting
	}

	// Determine the sField if the type if of geo.
	dfToWorkOn := ""
	if rsQuery.Type == querytranslate.Geo {
		// Try to extract the field on which the geo query was
		// made.
		sField, sFieldPresent := queryRun["sfield"]
		if !sFieldPresent {
			// Try to parse `fq`
			fq, fqPresent := queryRun["fq"]
			if fqPresent {
				fqSplitted := strings.Split(fq, ":")
				if len(fqSplitted) > 0 {
					dfToWorkOn = fqSplitted[0]
				}
			} else {
				// Try to extract from the `q` field.
				q, qPresent := queryRun["q"]
				if qPresent {
					qSplitted := strings.Split(q, ":")
					if len(qSplitted) > 0 {
						dfToWorkOn = qSplitted[0]
					}
				}
			}
		} else {
			// We will use sField to extract lat lon
			dfToWorkOn = sField
		}
	}

	shouldParseLatLon := dfToWorkOn != ""

	RSHitsArr := make([]interface{}, 0)
	for hitNo, hit := range actualHits {
		hitEach := make(map[string]interface{})

		hitAsMap, hitIdOk := hit.(map[string]interface{})
		if !hitIdOk {
			return nil, settingsToReturn, &Error{
				Err:  fmt.Errorf("Error while extracting ID from hit for hit number: %d", hitNo),
				Code: http.StatusInternalServerError,
			}
		}
		hitEach["_id"] = hitAsMap["id"]
		hitEach["_score"] = hitAsMap["score"]
		hitEach["_type"] = "_doc"

		// If lat lon needs to be extracted, try to extract it accordingly.
		//
		// In case of failure, we will silently ignore the error.
		if shouldParseLatLon {
			dfValue, dfOk := hitAsMap[dfToWorkOn].(string)
			if dfOk {
				lat, lon, latLonErr := extractCoordinatesFromString(dfValue)
				if latLonErr != nil {
					log.Warnln(logTag, ": error while trying to extract latitude and longitude to set it in the source body, ", latLonErr)
				} else {
					hitAsMap[dfToWorkOn] = map[string]interface{}{
						"lat": fmt.Sprintf("%f", lat),
						"lon": fmt.Sprintf("%f", lon),
					}
				}
			}
		}

		// Remove any fields from _source
		// Remove the `score` field since it should not be
		// present in the source
		delete(hitAsMap, "score")

		// Remove the `id` field if it's not present in includeFields.
		// This is an edge case because whatever fields are passed in
		// `includeFields` are appended with `id` and `score`.
		// TODO: As of now it's hard to determine the `includeFields` value in
		// the original request so keep the `id` field even if it's ignored.
		// delete(hitAsMap, "id")

		hitEach["_source"] = hitAsMap

		if isHighlightPresent {
			idAsString := hitEach["_id"].(string)

			highlightForId, highlightOk := highlightAsMap[idAsString]
			if !highlightOk {
				return nil, settingsToReturn, &Error{
					Err:  fmt.Errorf("Error while parsing highlight for ID: %s", idAsString),
					Code: http.StatusInternalServerError,
				}
			}

			hitEach["highlight"] = highlightForId
		}

		RSHitsArr = append(RSHitsArr, hitEach)
	}
	hitsMap["hits"] = RSHitsArr

	// Add the EachMap to RSResponse
	RSResponseEachMap["hits"] = hitsMap

	// If the type is suggestion than convert the hits
	// into SuggestionHit.
	if rsQuery.Type == querytranslate.Suggestion {

		// Set index suggestions as enabled if not passed
		//
		// TODO: Following should be removed when other flags
		// are supported
		if rsQuery.EnableIndexSuggestions == nil {
			indexSuggestions := true
			rsQuery.EnableIndexSuggestions = &indexSuggestions
		}

		// If indexSuggestions is disabled, then return an empty hits object.
		// NOTE: Following behavior should be changed when other suggestion
		// types are supported.
		suggestionHits := make([]querytranslate.SuggestionHIT, 0)
		if rsQuery.EnableIndexSuggestions != nil && *rsQuery.EnableIndexSuggestions {
			var suggestionExtractErr error
			suggestionHits, suggestionExtractErr = extractIndexSuggestions(RSHitsArr, rsQuery)
			if suggestionExtractErr != nil {
				errMsg := fmt.Sprint("error while extracting the suggestions from the response, ", suggestionExtractErr)
				log.Errorln(logTag, ": ", errMsg)

				return nil, settingsToReturn, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusInternalServerError,
				}
			}
		}

		hitsMap["hits"] = suggestionHits
		RSResponseEachMap["hits"] = hitsMap
		return RSResponseEachMap, settingsToReturn, nil
	}

	// Extract the aggregations if type is term or range
	// type will be considered term if `facet` is present in the queryRun
	// map
	if rsQuery.Type != querytranslate.Range && rsQuery.Type != querytranslate.Term {
		return RSResponseEachMap, settingsToReturn, nil
	}

	facetCounts, facetCountExists := responseAsMap["facet_counts"]
	if !facetCountExists {
		// Ignore, seems like facets are not present.
		return RSResponseEachMap, settingsToReturn, &Error{
			Err:  fmt.Errorf("error while extracting the facet_counts field"),
			Code: http.StatusInternalServerError,
		}
	}

	facetCountsAsMap, facetCountsOk := facetCounts.(map[string]interface{})
	if !facetCountsOk {
		return RSResponseEachMap, settingsToReturn, &Error{
			Err:  fmt.Errorf("error while parsing facet_counts to map"),
			Code: http.StatusInternalServerError,
		}
	}

	// Field name
	// The field name will differ depending on the type of query.
	//
	// The dataField name will be extracted from different fields depending
	// on type.
	fieldName := ""
	dataFieldUsed := ""
	pivotField := ""

	isPivotQuery := false

	_, isHistogramPresent := queryRun["facet.range"]

	switch rsQuery.Type {
	case querytranslate.Range:
		fieldName = "facet_ranges"
		dataFieldUsed, ok = queryRun["facet.range"]
		if !ok {
			dataFieldUsed = queryRun["stats.field"]
		}
		break
	case querytranslate.Term:
		var isPresent = false
		fieldName = "facet_fields"
		dataFieldUsed, isPresent = queryRun["facet.field"]

		// If `facet.field` is not present, it might be a
		// pivot facet query.
		if !isPresent {
			pivotField, isPivotQuery = queryRun["facet.pivot"]
			fieldName = "facet_pivot"
		}
	}

	// Logically this should never happen but still throw an
	// error just in case.
	if fieldName == "" {
		return RSResponseEachMap, settingsToReturn, &Error{
			Err:  fmt.Errorf("couldn't find a fieldName to extract from facet"),
			Code: http.StatusInternalServerError,
		}
	}

	// If it is a pivot facet, handle it directly
	if isPivotQuery {
		facetPivotAsMap, facetPivotOk := facetCountsAsMap[fieldName].(map[string]interface{})
		if !facetPivotOk {
			return RSResponseEachMap, settingsToReturn, &Error{
				Err:  fmt.Errorf("error while extracting facet_pivot to map"),
				Code: http.StatusInternalServerError,
			}
		}

		// Handle response for pivot facets properly.
		pivotBucketMap, pivotParseErr := extractPivotAggregations(facetPivotAsMap, pivotField)
		if pivotParseErr != nil {
			return RSResponseEachMap, settingsToReturn, pivotParseErr
		}

		// If no error was returned, return the aggregations properly.
		RSResponseEachMap["aggregations"] = pivotBucketMap
		return RSResponseEachMap, settingsToReturn, nil
	}

	aggrMapOutput := make(map[string]interface{})

	if isHistogramPresent || rsQuery.Type == querytranslate.Term {
		facetFieldsAsMap, facetFieldOk := facetCountsAsMap[fieldName].(map[string]interface{})
		if !facetFieldOk {
			return RSResponseEachMap, settingsToReturn, &Error{
				Err:  fmt.Errorf("error while extracting facet_fields to map"),
				Code: http.StatusInternalServerError,
			}
		}

		aggregationContent, aggregationOk := facetFieldsAsMap[dataFieldUsed]
		if !aggregationOk {
			return RSResponseEachMap, settingsToReturn, &Error{
				Err:  fmt.Errorf("error while extracting the aggregation data"),
				Code: http.StatusInternalServerError,
			}
		}

		// If type is range, we have one more layer before accessing the array.
		if rsQuery.Type == querytranslate.Range {
			aggregationContentAsMap, ok := aggregationContent.(map[string]interface{})
			if !ok {
				return RSResponseEachMap, settingsToReturn, &Error{
					Err:  fmt.Errorf("error while parsing aggregation content into map"),
					Code: http.StatusInternalServerError,
				}
			}

			aggregationContent = aggregationContentAsMap["counts"]
		}

		// Parse the aggregation data into RS equivalent.
		aggregationAsArr, aggrArrOk := aggregationContent.([]interface{})
		if !aggrArrOk {
			return RSResponseEachMap, settingsToReturn, &Error{
				Err:  fmt.Errorf("error while parsing aggregation content to array of interface"),
				Code: http.StatusInternalServerError,
			}
		}

		bucketsMap := make([]map[string]interface{}, 0)

		// If size is passed then set it accordingly.
		// Else set it to the fetched details array length
		if rsQuery.Size == nil {
			defaultValue := len(aggregationAsArr)
			rsQuery.Size = &defaultValue
		}

		// If aggregationSize is present, give it priority
		// over size.
		if rsQuery.AggregationSize != nil {
			rsQuery.Size = rsQuery.AggregationSize
		}

		// In the array the odd values are the field names
		// and the even values are the number of matches.
		for aggrIndex, _ := range aggregationAsArr {
			// If odd then skip it since when it's even
			// we will use the odd index as well.
			if aggrIndex%2 != 0 {
				continue
			}

			// There is no way to pass `size` field for facets
			// so we will have to do it ourselves.
			//
			// Break the loop when bucketsMap has length more than
			// or equal to the size passed.
			if len(bucketsMap) >= *rsQuery.Size {
				break
			}

			fieldName := aggregationAsArr[aggrIndex].(string)
			fieldCount := aggregationAsArr[aggrIndex+1].(float64)

			bucketMapEach := map[string]interface{}{
				"key":       fieldName,
				"doc_count": fieldCount,
			}

			bucketsMap = append(bucketsMap, bucketMapEach)
		}

		// If `selectAllLabel` is passed then parse the `hits.total.value` to a new
		// value in the bucket.
		if rsQuery.SelectAllLabel != nil {
			totalHits := responseHits["numFound"]
			bucketsMap = append(bucketsMap, map[string]interface{}{
				"key":       *rsQuery.SelectAllLabel,
				"doc_count": totalHits,
			})
		}

		aggrFieldMap := make(map[string]interface{})
		aggrFieldMap["buckets"] = bucketsMap

		aggrMapOutput[dataFieldUsed] = aggrFieldMap
	}

	// Extract min and max if it is present.
	if rsQuery.Aggregations != nil {
		isMinPresent, isMaxPresent, _ := checkAggregationValues(*rsQuery.Aggregations)

		if isMinPresent || isMaxPresent {
			statsAsMap, statsOk := responseAsMap["stats"].(map[string]interface{})
			if !statsOk {
				return RSResponseEachMap, settingsToReturn, &Error{
					Err:  fmt.Errorf("error while parsing stats from the response"),
					Code: http.StatusInternalServerError,
				}
			}

			statsFields, statsFieldsOk := statsAsMap["stats_fields"].(map[string]interface{})
			if !statsFieldsOk {
				return RSResponseEachMap, settingsToReturn, &Error{
					Err:  fmt.Errorf("error while parsing stats fields"),
					Code: http.StatusInternalServerError,
				}
			}

			dfStats, dfStatsOk := statsFields[dataFieldUsed].(map[string]interface{})
			if !dfStatsOk {
				return RSResponseEachMap, settingsToReturn, &Error{
					Err:  fmt.Errorf("error while extracting stats for field used"),
					Code: http.StatusInternalServerError,
				}
			}

			if isMinPresent {
				var minValue interface{}

				minValueAsInt, minValueOk := dfStats["min"].(float64)
				if !minValueOk {
					// Parse it to string and return
					minValueAsStr, minValueAsStrOk := dfStats["min"].(string)

					if !minValueAsStrOk {
						return RSResponseEachMap, settingsToReturn, &Error{
							Err:  fmt.Errorf("error while extracting min field for stats"),
							Code: http.StatusInternalServerError,
						}
					}

					minValue = minValueAsStr
				} else {
					minValue = minValueAsInt
				}

				minValueMap := map[string]interface{}{
					"value": minValue,
				}
				aggrMapOutput["min"] = minValueMap
			}

			if isMaxPresent {
				var maxValue interface{}

				maxValueAsInt, maxValueOk := dfStats["max"].(float64)
				if !maxValueOk {
					// Try to parse as string
					maxValueAsStr, maxValueAsStrOk := dfStats["max"].(string)
					if !maxValueAsStrOk {
						return RSResponseEachMap, settingsToReturn, &Error{
							Err:  fmt.Errorf("error while extracting max field for stats"),
							Code: http.StatusInternalServerError,
						}
					}

					maxValue = maxValueAsStr
				} else {
					maxValue = maxValueAsInt
				}

				maxValueMap := map[string]interface{}{
					"value": maxValue,
				}
				aggrMapOutput["max"] = maxValueMap
			}
		}
	}

	// Finally add the aggregations field
	RSResponseEachMap["aggregations"] = aggrMapOutput

	return RSResponseEachMap, settingsToReturn, nil
}

// TranslateToSolr will translate the passed query to Solr
// and return a string that can be used to pass in the Solr endpoint
func TranslateToSolr(rsQuery querytranslate.Query, allQueries *[]querytranslate.Query) (map[string]string, *Error) {
	queryMap := make(map[string]string)

	// Add `defType` as edismax by default
	queryMap["defType"] = "edismax"

	// Add index if it is present
	queryMap["_index"] = ""
	if rsQuery.Index != nil {
		queryMap["_index"] = *rsQuery.Index
	}

	if rsQuery.ID == nil {
		return queryMap, &Error{
			Err:  errors.New("field 'id' can't be empty"),
			Code: http.StatusBadRequest,
		}
	}

	// If `defaultQuery` is passed, there is no need to build a query, we
	// will use it directly.
	if rsQuery.DefaultQuery != nil {
		// Parse only if `query` is passed
		parsedQueryMap, parseErr := parsePassedQuery(rsQuery.DefaultQuery)
		if parseErr != nil {
			// Throw error
			return queryMap, parseErr
		}

		if len(parsedQueryMap) >= 1 {
			return parsedQueryMap, nil
		}
	}

	// Build the q value using dataField and value
	solrDFKey, solrDataField, conversionErr := RSToSolr["dataField"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}

	queryMap[solrDFKey] = solrDataField

	shouldRemoveRangeValueAndDf := rsQuery.Type == querytranslate.Range && (rsQuery.Size == nil || *rsQuery.Size == 0)

	// NOTE: If type is term or range, remove the dataField from the map since we don't
	// care about the hits at all
	// OR
	// If the query is of type `search` and value of `q` is `*` then no need
	// to setup `qf`.
	if rsQuery.Type == querytranslate.Term || shouldRemoveRangeValueAndDf || (rsQuery.Type == querytranslate.Search && (rsQuery.Value == nil || *rsQuery.Value == "")) {
		delete(queryMap, solrDFKey)
	}

	// If type is term, add special fields for it as well.
	// NOTE: It is important this translation happens after dataField is translated
	// because we are assuming that dataField will be of length 1 in case of
	// term and dataField conversion will enforce that.
	//
	// If type is term, remove the dataField from the map since we don't
	// care about the hits at all
	if rsQuery.Type == querytranslate.Term {
		queryMap["facet"] = "true"

		// Set the dataField as the facet.field
		//
		// If the dataField value contains a comma, it means it
		// is a pivot dataField so we will instead set the facet.pivot
		// field.
		if strings.Contains(solrDataField, ",") {
			queryMap["facet.pivot"] = solrDataField
		} else {
			queryMap["facet.field"] = solrDataField
		}

		// Delete the qf field
		delete(queryMap, solrDFKey)
	}

	includeValueKey, includeValueMap, includeValueErr := RSToSolr["includeValues"](&rsQuery, allQueries)
	if includeValueErr != nil {
		return queryMap, includeValueErr
	}

	// Parse the map into string array
	valuesAsStrArr := strings.Split(includeValueMap, "<-->")
	queryMap[includeValueKey] = strings.Join(valuesAsStrArr, "&facet.contains=")

	excludeValueKey, excludeValueMap, excludeValueErr := RSToSolr["excludeValues"](&rsQuery, allQueries)
	if excludeValueErr != nil {
		return queryMap, excludeValueErr
	}

	// Parse the map into string array
	excValuesAsStrArr := strings.Split(excludeValueMap, "<-->")
	queryMap[excludeValueKey] = strings.Join(excValuesAsStrArr, "&facet.excludeTerms=")

	if rsQuery.React != nil {
		reactKey, reactValue, reactErr := RSToSolr["react"](&rsQuery, allQueries)
		if reactErr != nil {
			return queryMap, reactErr
		}

		queryMap[reactKey] = reactValue
	}

	solrValueKey, solrValueField, conversionErr := RSToSolr["value"](&rsQuery, nil)
	log.Debug(logTag, ": value: ", solrValueField)
	if conversionErr != nil {
		return queryMap, conversionErr
	}

	queryMap[solrValueKey] = solrValueField

	// NOTE: If the type is `term`, we need to set `q` to * and ignore generating
	// the value altogether.
	if rsQuery.Type == querytranslate.Term {
		queryMap["q"] = "*"
	}

	// Sometimes the value can be `fq`
	// In such a case, we need to set the `q` param to `*`
	//
	// When `fq` is returned, in a lot of cases, there can be multiple params
	// in the same string so we need to break them out.
	if solrValueKey == "fq" {
		queryMap["q"] = "*"

		splittedValue := strings.Split(solrValueField, "&")
		// If the length is 1, it means there is no & in the string
		if len(splittedValue) > 1 {
			for index, value := range splittedValue {
				// The first value will belong to `fq` here
				if index == 0 {
					queryMap["fq"] = value
					continue
				}

				valueSplitted := strings.Split(value, "=")
				if len(valueSplitted) != 2 {
					continue
				}

				key := valueSplitted[0]
				mappedValue := valueSplitted[1]

				// Add the values to the queryMap
				queryMap[key] = mappedValue
			}
		}
	}

	// If type is range and aggregations is passed, we need to set some fields accordingly.
	//
	// Aggregations can contain three fields:
	// - histogram: should be general range facet on a field that is numeric.
	// - min: minimum value of a field.
	// - max: maximum value of a field.
	//
	// If `min` or `max` is present, we will pass the stats field.
	if rsQuery.Type == querytranslate.Range && rsQuery.Aggregations != nil {
		isMinPresent, isMaxPresent, isHistogramPresent := checkAggregationValues(*rsQuery.Aggregations)

		isMinMaxPresent := isMinPresent || isMaxPresent

		// The dataField should be either the parsed dataField
		// or the first value in the dataField array.
		dataFieldSplitted := strings.Split(solrDataField, " ")
		dataFieldToUse := dataFieldSplitted[0]

		// If min/max is present, we need to add the stats field.
		if isMinMaxPresent {
			queryMap["stats"] = "true"
			queryMap["stats.field"] = dataFieldToUse
		}

		if isHistogramPresent {
			// Extract the start and end values from the query.
			//
			// NOTE: Assumption for the value field is that it
			// will be of type `[%s TO %s]` so we need to find the
			// start and end from there.
			valueSplitted := strings.Split(regexp.MustCompile(`\[|\]|\s`).ReplaceAllString(solrValueField, ""), "TO")
			start := valueSplitted[0]
			end := valueSplitted[1]

			var gapToUse interface{}

			defaultGapToUse := 1
			gapToUse = defaultGapToUse

			// If `start` and `end` are of type date, then we need to set the default
			// interval in a different way.
			_, isStartAsDate := time.Parse("2006-01-02T15:04:05Z", start)
			// There is a possibility that the date might be passed without timezone,
			// in such a case, we need to attach a 00:00:00 timezone.
			_, isStartWithoutTzDate := time.Parse("2006-01-02", start)

			if isStartAsDate == nil || isStartWithoutTzDate == nil {
				if isStartWithoutTzDate == nil {
					// It is a date passed without timezone, we need to attach the
					// timezone
					start = fmt.Sprintf("%sT00:00:00Z", start)
					end = fmt.Sprintf("%sT00:00:00Z", end)
				}

				_, calenderIntervalToUse, calenderIntervalErr := RSToSolr["calendarinterval"](&rsQuery, nil)
				if calenderIntervalErr != nil {
					return queryMap, calenderIntervalErr
				}

				queryMap["facet.range.gap"] = calenderIntervalToUse

			} else {
				// Else the values are of int type so we will calculate interval
				// using math.
				// Calculate the interval field

				if rsQuery.Interval == nil {
					if start == "__min__" {
						// Should be resolved later.
						gapToUse = -1
					}

					startAsFloat, startAsFloatOK := strconv.ParseFloat(start, 64)
					endAsFloat, endAsFloatOK := strconv.ParseFloat(end, 64)

					if startAsFloatOK != nil || endAsFloatOK != nil {
						return queryMap, &Error{
							Err:  fmt.Errorf("Error while reading `start` or `end` in value to calculate default interval"),
							Code: http.StatusBadRequest,
						}

					}

					intervalToUse := int(math.Ceil((endAsFloat - startAsFloat) / 100))
					if intervalToUse <= 0 {
						gapToUse = 1
					} else {
						gapToUse = intervalToUse
					}
				} else {
					gapToUse = *rsQuery.Interval
				}

				queryMap["facet.range.gap"] = fmt.Sprintf("%d", gapToUse)
			}

			queryMap["facet.range"] = dataFieldToUse
			queryMap["facet.range.start"] = start
			queryMap["facet.range.end"] = end

		}
	}

	// NOTE: Make this deletion happen after the above `isHistogramPresent` check
	// because that depends on the value.
	if shouldRemoveRangeValueAndDf {
		delete(queryMap, "q")
	}

	// Build the q value since queryFormat modifies it.
	// This will modify q only if the type is of suggestion or search
	solrValueKey, solrValueVal, conversionErr := RSToSolr["queryFormat"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}
	queryMap[solrValueKey] = solrValueVal

	// Build the size
	solrSizeKey, solrSize, conversionErr := RSToSolr["size"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}

	queryMap[solrSizeKey] = solrSize

	// Build the from value
	solrFromKey, solrFrom, conversionErr := RSToSolr["from"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}

	queryMap[solrFromKey] = solrFrom

	// Build the includeFields value
	solrIncludeFieldsKey, solrIncludeFieldsValue, conversionErr := RSToSolr["includeFields"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}

	queryMap[solrIncludeFieldsKey] = solrIncludeFieldsValue

	// Build the sortBy value
	solrSortKey, solrSortValue, conversionErr := RSToSolr["sortBy"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}

	queryMap[solrSortKey] = solrSortValue

	// NOTE: If solrSortKey is `facet.sort` we need to add the sort field separately.
	// This is because cursor requires a valid sortField to be specified.
	if solrSortKey == "facet.sort" {
		queryMap["sort"] = "id asc"
	}

	// Build the deepPagination value
	solrDPKey, solrDPValue, conversionErr := RSToSolr["deepPagination"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}

	queryMap[solrDPKey] = solrDPValue

	// Parse the highlight field
	solrHighlightKey, solrHighlightValue, conversionErr := RSToSolr["highlight"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}
	queryMap[solrHighlightKey] = solrHighlightValue

	// Parse the highlightConfig field.
	// This field is special because it maps to multiple fields
	// in Solr.
	// The response strings are joined by the special string `<-->`
	// We will need to split it based on that and accordingly make it
	// work.
	solrHLConfKeys, solrHLConfValues, conversionErr := RSToSolr["highlightConfig"](&rsQuery, nil)
	if conversionErr != nil {
		return queryMap, conversionErr
	}
	solrHLKeysSplitted := strings.Split(solrHLConfKeys, "<-->")
	solrHLValuesSplitted := strings.Split(solrHLConfValues, "<-->")

	for index := range solrHLKeysSplitted {
		key := solrHLKeysSplitted[index]
		value := solrHLValuesSplitted[index]

		queryMap[key] = value
	}

	return queryMap, nil
}

// extractPivotAggregations will extract the aggregations for pivot
// facets.
//
// This function will take care of extracting as many layers of facets
// as specified in the request.
func extractPivotAggregations(facetPivotAsMap map[string]interface{}, pivotField string) (map[string]interface{}, *Error) {
	topLevelBucket, topLevelBucketOk := facetPivotAsMap[pivotField].([]interface{})
	if !topLevelBucketOk {
		return nil, &Error{
			Err:  fmt.Errorf("error while parsing top level bucket for pivot facets"),
			Code: http.StatusInternalServerError,
		}
	}

	// Convert the array of interface to an array of map
	topLevelBucketAsMap := make([]map[string]interface{}, 0)
	for topLevelBucketIndex, topLevelBucketEach := range topLevelBucket {
		topLevelBucketEachAsMap, asMapOk := topLevelBucketEach.(map[string]interface{})
		if !asMapOk {
			return nil, &Error{
				Err:  fmt.Errorf("error while parsing top level bucket to map at index: %d", topLevelBucketIndex),
				Code: http.StatusInternalServerError,
			}
		}

		topLevelBucketAsMap = append(topLevelBucketAsMap, topLevelBucketEachAsMap)
	}

	topLevelBucketParsed, topLevelField, bucketParseErr := pivotToBucket(topLevelBucketAsMap)
	if bucketParseErr != nil {
		return nil, bucketParseErr
	}

	topLevelMap := map[string]interface{}{
		topLevelField: map[string]interface{}{
			"buckets": topLevelBucketParsed,
		},
	}
	return topLevelMap, nil
}

// pivotToBucket will parse the pivot array into a bucket array
// and accordingly return the values.
func pivotToBucket(pivotArr []map[string]interface{}) ([]map[string]interface{}, string, *Error) {
	bucketArr := make([]map[string]interface{}, 0)

	fieldToReturn := ""

	for pivotIndex, pivotEach := range pivotArr {
		bucketEach := map[string]interface{}{
			"key":       pivotEach["value"],
			"doc_count": pivotEach["count"],
		}

		if fieldToReturn == "" {
			fieldToReturn = pivotEach["field"].(string)
		}

		// If `pivot` is present then we need to parse that as well.
		pivotBucket, isBucketPresent := pivotEach["pivot"]
		if !isBucketPresent {
			bucketArr = append(bucketArr, bucketEach)
			continue
		}

		// If bucket is present, we need to parse the contents.

		bucketAsArr, asArrOk := pivotBucket.([]interface{})
		if !asArrOk {
			return bucketArr, "", &Error{
				Err:  fmt.Errorf("error while parsing bucket to array of interfaces for %s at index: %d", pivotEach["value"], pivotIndex),
				Code: http.StatusInternalServerError,
			}
		}

		// Parse the bucket into an array of map[string]interface
		bucketAsMapArr := make([]map[string]interface{}, 0)
		for bucketIndex, bucketAsInterface := range bucketAsArr {
			bucketEachAsMap, asMapOk := bucketAsInterface.(map[string]interface{})
			if !asMapOk {
				return bucketArr, "", &Error{
					Err:  fmt.Errorf("error while parsing the bucket at index: %d to a map from interface", bucketIndex),
					Code: http.StatusInternalServerError,
				}
			}

			bucketAsMapArr = append(bucketAsMapArr, bucketEachAsMap)
		}

		pivotBucketToBucket, fieldToSet, bucketParseErr := pivotToBucket(bucketAsMapArr)
		if bucketParseErr != nil {
			return bucketArr, "", &Error{
				Err:  fmt.Errorf("error while parsing nested bucket for %s at index: %d", pivotEach["value"], pivotIndex),
				Code: http.StatusInternalServerError,
			}
		}

		// Set the bucket arr now
		bucketEach[fieldToSet] = map[string]interface{}{
			"buckets": pivotBucketToBucket,
		}
		bucketArr = append(bucketArr, bucketEach)
	}

	return bucketArr, fieldToReturn, nil
}

// extractIndexSuggestions will extract the index suggestions
// for the passed response and accordingly return a response
// that can be returned back directly
func extractIndexSuggestions(hits []interface{}, rsQuery querytranslate.Query) ([]querytranslate.SuggestionHIT, error) {
	// Convert the hits into an ESDoc array
	esDocHits := make([]querytranslate.ESDoc, 0)
	for _, hit := range hits {
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

	// Parse the value into a string.

	if rsQuery.Value == nil {
		defaultValue := new(interface{})
		*defaultValue = "*"
		rsQuery.Value = defaultValue
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

	// Update the value and label fields
	for hitIndex, suggestionHit := range suggestionHits {
		// Extract the `term_s` or `term_t` from the source and replace the
		// `value` and `label` fields with it.
		finalValueToUse := ""

		termS, termSOk := suggestionHit.Source["term_s"]
		if termSOk {
			// Try to parse it to string and use it.
			termSAsString, asStrOk := termS.(string)
			if asStrOk {
				finalValueToUse = termSAsString
			}
		} else {
			// Try to extract the term_t field
			termT, termTOk := suggestionHit.Source["term_t"]
			if termTOk {
				termTAsStr, asStrOk := termT.(string)
				if asStrOk {
					finalValueToUse = termTAsStr
				}
			}
		}

		if finalValueToUse != "" {
			suggestionHits[hitIndex].Value = finalValueToUse
			suggestionHits[hitIndex].Label = finalValueToUse
		}
	}

	return suggestionHits, nil
}

// convertDataField will return a function that will
// convert the `dataField` and make it Solr compatible
//
// dataField will map to qf in solr
func convertDataField() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		// Try to parse it as an array of objects
		normalizedFields := querytranslate.NormalizedDataFields(query.DataField, query.FieldWeights)

		solrDataFields := make([]string, 0)

		// Iterate over the fields and build the array of strings
		// by considering the field weights and the fieldnames
		for _, field := range normalizedFields {
			solrDF := field.Field

			// NOTE: _score is a special field that needs to be ignored for
			// dF
			if solrDF == "_score" {
				continue
			}

			if field.Weight != 0 {
				solrDF += fmt.Sprintf("%s^%f", solrDF, field.Weight)
			}

			solrDataFields = append(solrDataFields, solrDF)
		}

		// Determine how to join the dataFields.
		//
		// For term type of queries, we want them to be joined by a comma (,)
		joinStringWith := " "
		if query.Type == querytranslate.Term && len(normalizedFields) > 1 {
			joinStringWith = ","
		}

		return "qf", strings.Join(solrDataFields, joinStringWith), nil
	}
}

// convertValue will return a function that will convert
// the `value` field and make it Solr compatible.
//
// value will map to q in Solr
func convertValue() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		// TODO: Handle other types of search types as well.
		// As of now, the assumption is that the search type is only
		// `search` and `suggestion`.

		// If value is not passed, set it to an empty string
		if query.Value == nil {
			if query.Type == querytranslate.Range {
				return "q", "[__min__ TO __max__]", nil
			}

			defaultQueryValue := new(interface{})
			*defaultQueryValue = "*"
			query.Value = defaultQueryValue
		}

		// If the value is of type array, we just need to check if queryFormat is set
		// If not set, we can set it and return from this function.
		// This is because, queryFormat will take care of building the query in case of
		// value being a string array.
		//
		// This condition should only be for search type of queries
		if query.Type == querytranslate.Search {
			_, valueAsArrOk := (*query.Value).([]interface{})
			if valueAsArrOk {
				if query.QueryFormat == nil {
					defaultQueryFormat := "OR"
					query.QueryFormat = &defaultQueryFormat
				}

				return "", "", nil
			}
		}

		if query.Type == querytranslate.Suggestion || query.Type == querytranslate.Search {
			normalizedQueryValue, normalizationErr := querytranslate.NormalizeQueryValue(query.Value)
			if normalizationErr != nil {
				// Handle error
				return "", "", &Error{
					Err:  normalizationErr,
					Code: http.StatusBadRequest,
				}
			}

			// Check if query value is not nil
			if normalizedQueryValue == nil {
				return "", "", &Error{
					Err:  fmt.Errorf("normalized query value is nil, is the query value passed? Is it passed in a proper format?"),
					Code: http.StatusBadRequest,
				}
			}

			queryValue := *normalizedQueryValue
			queryString := queryValue.(string)

			// Add a postfix `*` at the end of the query
			log.Debug(logTag, ": value: ", queryString)
			if (query.Type == querytranslate.Suggestion || query.Type == querytranslate.Search) && len(queryString) > 0 && queryString[len(queryString)-1:] != "*" {
				queryString += "*"
			}

			// Update the value of `value` in the passed query since it will be accessed
			// later on
			var queryAsInterface interface{}
			queryAsInterface = queryString
			query.Value = &queryAsInterface

			return "q", queryString, nil
		}

		// Parse the value field for range
		if query.Type == querytranslate.Range {
			rangeValue, rangeValueErr := query.GetRangeValue(*query.Value)
			if rangeValueErr != nil {
				return "", "", &Error{
					Err:  rangeValueErr,
					Code: http.StatusBadRequest,
				}
			}

			// Convert the start to an interface or string accordingly
			start, startErr := parseRangeValue(*rangeValue.Start)
			if startErr != nil {
				return "", "", &Error{
					Err:  fmt.Errorf("`start`: %s", startErr.Error()),
					Code: http.StatusBadRequest,
				}
			}

			end, endErr := parseRangeValue(*rangeValue.End)
			if endErr != nil {
				return "", "", &Error{
					Err:  fmt.Errorf("`end`: %s", endErr.Error()),
					Code: http.StatusBadRequest,
				}
			}

			// TODO: How should boost be used?
			return "q", fmt.Sprintf("[%s TO %s]", start, end), nil
		}

		// Parse the value field for a geo type of query
		if query.Type == querytranslate.Geo {

			// Parse the dataField
			//
			// In case it's a map or array, we use the first
			// value.
			sfieldToUse := ""

			dfPassed := query.DataField

			dfAsMap, dfAsMapOk := dfPassed.(map[string]interface{})
			if dfAsMapOk {
				for df, _ := range dfAsMap {
					sfieldToUse = df
					break
				}
			} else {
				// Parse as array
				dfAsArr, dfAsArrOk := dfPassed.([]interface{})
				if dfAsArrOk {
					for dfIndex, df := range dfAsArr {
						sfieldAsStr, strOk := df.(string)
						if !strOk {
							return "", "", &Error{
								Err:  fmt.Errorf("invalid value passed at index: %d for dataField for ID: %s", dfIndex, *query.ID),
								Code: http.StatusBadRequest,
							}
						}
						sfieldToUse = sfieldAsStr
					}
				} else {
					dfAsStr, dfAsStrOk := dfPassed.(string)

					if !dfAsStrOk {
						return "", "", &Error{
							Err:  fmt.Errorf("couldn't parse dataField to use in value for ID: %s", *query.ID),
							Code: http.StatusBadRequest,
						}
					}

					sfieldToUse = dfAsStr
				}
			}

			if *query.Value == "*" {
				// Check if value is not passed and react is present, in such a case, treat the
				// type as search.
				if query.React != nil {
					query.Type = querytranslate.Search
				}
				return "q", fmt.Sprintf("%s:%s", sfieldToUse, "*"), nil
			}

			geoValue, geoValueErr := query.GetSolrGeoValue()

			if geoValueErr != nil {
				return "", "", &Error{
					Err:  fmt.Errorf("invalid value passed for `value` and type `geo`: %f", geoValueErr),
					Code: http.StatusBadRequest,
				}
			}

			// Parse the location, distance and unit if passed
			if geoValue.Location != nil && geoValue.Distance != nil && geoValue.Unit != nil {
				// Only `km` is a valid unit type in Solr, however, we support
				// a range of different units.
				// This is why we will convert the distance based on the passed
				// unit into `km`'s before using it.
				convertedDistance, convertErr := convertDistanceForUnit(*geoValue.Distance, *geoValue.Unit)
				if convertErr != nil {
					return "", "", &Error{
						Err:  convertErr,
						Code: http.StatusBadRequest,
					}
				}

				return "fq", fmt.Sprintf("{!geofilt}&sfield=%s&pt=%s&d=%f", sfieldToUse, *geoValue.Location, convertedDistance), nil
			} else if geoValue.BoundingBox != nil {
				bottomLeft, topRight, oppositeCoordinateErr := calculateOppositeCoordinates(geoValue.BoundingBox.TopLeft, geoValue.BoundingBox.BottomRight)
				if oppositeCoordinateErr != nil {
					return "", "", oppositeCoordinateErr
				}

				return "fq", fmt.Sprintf("%s:[%s TO %s]", sfieldToUse, bottomLeft, topRight), nil
			}

			// If execution reaches here, neither value or BBox was valid.
			return "", "", &Error{
				Err:  fmt.Errorf("invalid value passed for `geo`"),
				Code: http.StatusBadRequest,
			}
		}

		// Else query should be for term
		//
		// Term can accept both array of string or a string.
		// We will try to parse to array of string first.

		// Determine the value of selectAllLabel
		selectAllLabelValue := ""
		if query.SelectAllLabel != nil {
			selectAllLabelValue = *query.SelectAllLabel
		}

		queryAsArr, asArrOk := (*query.Value).([]interface{})
		if asArrOk {
			// Parse into array of strings
			queryAsStrArr := make([]string, 0)

			if len(queryAsArr) <= 0 {
				queryAsArr = append(queryAsArr, "*")
			}

			for queryIndex, queryValue := range queryAsArr {
				queryStr, asStrOk := queryValue.(string)
				if !asStrOk {
					return "", "", &Error{
						Err:  fmt.Errorf("error while parsing the array of interface to string array for value"),
						Code: http.StatusInternalServerError,
					}
				}

				// See if selectAllLabel matches it
				if selectAllLabelValue == queryStr {
					queryAsArr[queryIndex] = "*"
				}

				queryAsStrArr = append(queryAsStrArr, queryStr)
			}

			*query.Value = queryAsArr

			_, queryFormatToUse, queryFormatErr := convertQueryFormat()(query, allQueries)
			if queryFormatErr != nil {
				return "", "", &Error{
					Err:  fmt.Errorf("error while parsing queryFormat to use for joining value for term query with err: %s", queryFormatErr.Err.Error()),
					Code: http.StatusBadRequest,
				}
			}

			return "q", queryFormatToUse, nil
		}

		queryAsString, asStrOk := (*query.Value).(string)
		if !asStrOk {
			return "", "", &Error{
				Err:  fmt.Errorf("value should be of type string for `term` queries"),
				Code: http.StatusBadRequest,
			}
		}

		// See if selectAllLabel matches it
		if selectAllLabelValue == queryAsString {
			queryAsString = "*"
			*query.Value = "*"
		}

		return "q", queryAsString, nil

	}
}

// convertSize will return a function that will convert
// the `size` field and make it Solr compatible
//
// size will map to rows in Solr.
func convertSize() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		solrSize := 10

		// Set results to be returned to 0 if term type is passed
		if query.Type == querytranslate.Term {
			solrSize = 0
		}

		if query.Size != nil {
			solrSize = *query.Size
		}

		return "rows", strconv.Itoa(solrSize), nil
	}
}

// convertFrom will return a function that will convert
// the `from` field and make it Solr compatible
//
// from will map to start in Solr
func convertFrom() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		solrStart := 0

		if query.From != nil {
			solrStart = *query.From
		}

		return "start", strconv.Itoa(solrStart), nil
	}
}

// convertIncludeFields will convert the `includeFields`
// field to it's Solr equivalent
//
// includeFields will map to fl in Solr
func convertIncludeFields() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		if query.IncludeFields == nil {
			return "fl", "score,*", nil
		}

		// Extract the fields and use them.
		builtFl := strings.Join(*query.IncludeFields, ",")
		builtFl += ",score,id"

		return "fl", builtFl, nil
	}
}

// convertSortBy will convert the sortBy field to its
// Solr equivalent.
//
// `sortBy` will map to `sort` in Solr.
func convertSortBy() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {

		defaultSortField := "score"

		// Set the dataField as sortField only if sortBy is passed by the user.
		if query.SortField == nil && query.DataField != nil && query.SortBy != nil {
			query.SortField = new(interface{})

			// Parse the dataField and set the sortField accordingly.
			dataFieldAsObject, objOk := query.DataField.(map[string]interface{})
			if objOk {
				dfValue := dataFieldAsObject["field"].(string)
				*query.SortField = dfValue
			} else {
				// Parse it as an array of objects
				dfAsArr, asArrOk := query.DataField.([]interface{})
				if asArrOk && len(dfAsArr) > 0 {
					// The first value can be both an object or string.
					valueAsObj, asObjOk := dfAsArr[0].(map[string]interface{})
					if asObjOk {
						dfValue := valueAsObj["field"].(string)
						*query.SortField = dfValue
					} else {
						// Will be a string
						dfValueAsStr := dfAsArr[0].(string)
						*query.SortField = dfValueAsStr
					}
				} else {
					// Parse as string since it will be a string
					dfAsStr, asStrOk := query.DataField.(string)
					if asStrOk {
						*query.SortField = dfAsStr
					}
				}
			}
		}

		// If df was not passed, it will still be nil
		isDefaultSet := false
		if query.SortField == nil {
			query.SortField = new(interface{})
			*query.SortField = defaultSortField
			isDefaultSet = true
		}

		if query.SortBy == nil {
			defaultSortBy := querytranslate.Desc
			query.SortBy = &defaultSortBy
		}

		if query.Type != querytranslate.Term && query.SortBy != nil && *query.SortBy == querytranslate.Count {
			// NOTE: Don't throw error here if the type is
			// not term
			//
			// Fallback to score desc
			log.Warnln(logTag, ": `count` is not supported for sort and falling back to score desc.")
			return "sort", "score desc", nil
		}

		// If type is term, then return facet sort
		if query.Type == querytranslate.Term {
			sortByToUse := query.SortBy.String()

			// Asc and Desc are not supported so set it to `count` which is default.
			if query.SortBy != nil && *query.SortBy == querytranslate.Desc {
				log.Warnln(logTag, ": `desc` is not supported in sortBy for `term` type of queries")
				sortByToUse = querytranslate.Count.String()
			}

			// `asc` maps to `index` type of sort for facets in Solr so map it accordingly.
			if query.SortBy != nil && *query.SortBy == querytranslate.Asc {
				sortByToUse = "index"
			}

			return "facet.sort", sortByToUse, nil
		}

		// Extract the sortField value passed.
		sortFieldParsed, sortFieldParseErr := querytranslate.ParseSortField(*query, *query.SortBy)
		if sortFieldParseErr != nil {
			log.Warnln(logTag, ": error while parsing sortField: ", sortFieldParseErr)
			return "", "", &Error{
				Err:  sortFieldParseErr,
				Code: http.StatusBadRequest,
			}
		}

		sortValueInArr := make([]string, 0)

		for field, order := range sortFieldParsed {
			// If the field is _score, it will be changed to score since `_score`
			// is a special field.
			if field == "_score" {
				field = "score"
			}
			sortValueInArr = append(sortValueInArr, fmt.Sprintf("%s %s", field, order.String()))
		}

		// If default value was not set, we need to append the score as asc
		if !isDefaultSet {
			sortValueInArr = append(sortValueInArr, "id asc")
		}

		return "sort", strings.Join(sortValueInArr, ", "), nil
	}
}

// convertQueryFormat will convert the queryFormat field to its
// Solr equivalent.
//
// `queryFormat` will map to a special `q` value in Solr.
//
// This method should be called only if queryFormat is passed, else
// an error will be thrown.
func convertQueryFormat() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		defaultQF := "or"
		if query.QueryFormat == nil {
			query.QueryFormat = &defaultQF
		}

		loweredQueryFormat := strings.ToLower(*query.QueryFormat)

		// Make sure only suggestion and search are supported since qf will be
		// ignored in range.
		if query.Type != querytranslate.Search && query.Type != querytranslate.Suggestion && query.Type != querytranslate.Term {
			// No need to thrown an error, just silently ignore it with a warning
			log.Warnln(logTag, "`queryFormat` is supported for only `search`, `term` and `suggestion` type of queries")
			return "", "", nil
		}

		// Make sure supported values are passed.
		// NOTE: This logic should change when range is also supported for Solr
		// queryFormat.
		if loweredQueryFormat != "or" && loweredQueryFormat != "and" {
			return "", "", &Error{
				Err:  fmt.Errorf("only `and` and `or` are supported for `queryFormat` in case of Solr"),
				Code: http.StatusBadRequest,
			}
		}

		// Set query value as * if not present
		if query.Value == nil || *query.Value == "" {
			defaultValue := new(interface{})
			*defaultValue = "*"
			query.Value = defaultValue
		}

		// Support both array of interface as well as string value
		splittedQuery := make([]string, 0)

		valueAsArr, asArrOk := (*query.Value).([]interface{})
		if !asArrOk {
			// Try to parse as string
			valueAsStr, asStrOk := (*query.Value).(string)
			if !asStrOk {
				// Report an error since invalid type is passed.
				return "", "", &Error{
					Err:  fmt.Errorf("invalid value passed for `query.value` for query with ID: %s", *query.ID),
					Code: http.StatusBadRequest,
				}
			}

			// Normalize the query value
			valueAsInterface := new(interface{})
			*valueAsInterface = valueAsStr
			normalizedQueryValue, normalizationErr := querytranslate.NormalizeQueryValue(valueAsInterface)
			if normalizationErr != nil {
				// Handle error
				return "", "", &Error{
					Err:  normalizationErr,
					Code: http.StatusBadRequest,
				}
			}

			// Split the string using space
			//
			// If the query is of type term, don't split it and use it as a whole without any queryformat
			if query.Type == querytranslate.Term {
				splittedQuery = append(splittedQuery, (*normalizedQueryValue).(string))
			} else {
				splittedString := strings.Split((*normalizedQueryValue).(string), " ")
				splittedQuery = append(splittedQuery, splittedString...)
			}

		} else {
			// If array is empty, add * as the only item
			if len(valueAsArr) < 1 {
				valueAsArr = append(valueAsArr, "*")
			}

			// If length is 1 and the first element is an empty string,
			// we need to replace it with an array of *
			if len(valueAsArr) == 1 {
				firstElAsStr, asStrOk := valueAsArr[0].(string)
				if !asStrOk {
					return "", "", &Error{
						Err:  fmt.Errorf("non-string value passed in `query.value` at index: 0"),
						Code: http.StatusBadRequest,
					}
				}

				if firstElAsStr == "" {
					valueAsArr[0] = "*"
				}
			}

			// Parse the array into string array
			for arrIndex, arrValue := range valueAsArr {
				arrValueAsStr, asStrOk := arrValue.(string)
				if !asStrOk {
					// Report the issue since only string array is supported.
					return "", "", &Error{
						Err:  fmt.Errorf("non-string value passed in `query.value` at index: %d", arrIndex),
						Code: http.StatusBadRequest,
					}
				}

				splittedQuery = append(splittedQuery, arrValueAsStr)
			}
		}

		switch loweredQueryFormat {
		case "or":
			return "q", strings.Join(splittedQuery, fmt.Sprintf(" %s ", "OR")), nil
		case "and":
			return "q", strings.Join(splittedQuery, fmt.Sprintf(" %s ", "AND")), nil
		default:
			return "", "", nil
		}
	}
}

// convertHighlight will convert the `highlight` field to its
// Solr equivalent.
//
// highlight will map to `hl` in Solr
func convertHighlight() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		shouldHighlight := false

		if query.Highlight != nil {
			shouldHighlight = *query.Highlight
		}

		return "hl", strconv.FormatBool(shouldHighlight), nil
	}
}

// convertHighlightConfig will convert the `highlightConfig` field
// to its Solr equivalent
//
// `highlightConfig` will map to various fields like pre, post etc.
//
// This method will throw an error if `highlight` is not passed by the
// user or is disabled.
func convertHighlightConfig() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		// Check if highlight is passed.
		if (query.Highlight == nil || !*query.Highlight) && query.HighlightConfig != nil {
			return "", "", &Error{
				Err:  fmt.Errorf("`highlightConfig` will not work if `highlight` is not passed or disabled for ID: %s", *query.ID),
				Code: http.StatusBadRequest,
			}
		}

		// Apply the default config if not passed.
		if query.HighlightConfig == nil {
			defaultHighlight := map[string]interface{}{
				"fields": map[string]interface{}{
					"*": map[string]interface{}{},
				},
			}
			query.HighlightConfig = &defaultHighlight
		}

		field, fieldOk := (*query.HighlightConfig)["fields"]
		preTags, preOk := (*query.HighlightConfig)["pre_tags"]
		postTags, postOk := (*query.HighlightConfig)["post_tags"]
		reqFieldMatch, fieldMatchOk := (*query.HighlightConfig)["require_field_match"]

		keysToReturn, valuesToReturn := make([]string, 0), make([]string, 0)

		// Field will map to hl.fl in Solr query.
		if fieldOk {
			// Convert to a map[string]interface{}
			fieldAsMap, asMapOk := field.(map[string]interface{})
			if !asMapOk {
				return "", "", &Error{
					Err:  fmt.Errorf("`highlightConfig.fields` needs to be a JSON object for query with ID: %s", *query.ID),
					Code: http.StatusBadRequest,
				}
			}

			fieldList := make([]string, 0)
			for field, _ := range fieldAsMap {
				fieldList = append(fieldList, field)
			}

			keysToReturn = append(keysToReturn, "hl.fl")
			valuesToReturn = append(valuesToReturn, strings.Join(fieldList, " "))
		}

		// reqFieldMatch will match to hl.requireFieldMatch in Solr.
		if fieldMatchOk {
			// Convert the value to a boolean
			fieldMatchAsBool, reqFieldMatchOk := reqFieldMatch.(bool)
			if !reqFieldMatchOk {
				return "", "", &Error{
					Err:  fmt.Errorf("`highlightConfig.require_field_match` needs to be a Boolean for query with ID: %s", *query.ID),
					Code: http.StatusBadRequest,
				}
			}

			keysToReturn = append(keysToReturn, "hl.requireFieldMatch")
			valuesToReturn = append(valuesToReturn, strconv.FormatBool(fieldMatchAsBool))
		}

		// pre_tag will map to hl.simple.pre
		if preOk {
			// Convert the value to an array
			preAsArray, preAsArrayOk := preTags.([]interface{})
			if !preAsArrayOk {
				return "", "", &Error{
					Err:  fmt.Errorf("`highlightConfig.pre_tags` needs to be an array of strings for query with ID: %s", *query.ID),
					Code: http.StatusBadRequest,
				}
			}

			preTagsStrArr := make([]string, 0)

			for preTagPosition, preTag := range preAsArray {
				// Parse to string
				preTagAsStr, asStrOk := preTag.(string)
				if !asStrOk {
					return "", "", &Error{
						Err:  fmt.Errorf("`highlightConfig.pre_tags` at position %d is not a string for query with ID: %s", preTagPosition, *query.ID),
						Code: http.StatusBadRequest,
					}
				}

				preTagsStrArr = append(preTagsStrArr, preTagAsStr)
			}

			// Join the tags and save them.
			keysToReturn = append(keysToReturn, "hl.simple.pre")
			valuesToReturn = append(valuesToReturn, strings.Join(preTagsStrArr, ""))
		}

		// post_tag will map to hl.post.simple
		if postOk {
			// Convert to array of strings
			// Convert the value to an array
			postAsArray, postAsArrayOk := postTags.([]interface{})
			if !postAsArrayOk {
				return "", "", &Error{
					Err:  fmt.Errorf("`highlightConfig.post_tags` needs to be an array of strings for query with ID: %s", *query.ID),
					Code: http.StatusBadRequest,
				}
			}

			postTagsStrArr := make([]string, 0)

			for postTagPosition, postTag := range postAsArray {
				// Parse to string
				postTagAsStr, asStrOk := postTag.(string)
				if !asStrOk {
					return "", "", &Error{
						Err:  fmt.Errorf("`highlightConfig.post_tags` at position %d is not a string for query with ID: %s", postTagPosition, *query.ID),
						Code: http.StatusBadRequest,
					}
				}

				postTagsStrArr = append(postTagsStrArr, postTagAsStr)
			}

			// Append the values now
			keysToReturn = append(keysToReturn, "hl.simple.post")
			valuesToReturn = append(valuesToReturn, strings.Join(postTagsStrArr, ""))
		}

		return strings.Join(keysToReturn, "<-->"), strings.Join(valuesToReturn, "<-->"), nil
	}
}

// convertCalendarInterval will convert the calenderInterval value
// to Solr equivalent.
//
// This value will be passed in the ES format as described here:
// https://www.elastic.co/guide/en/elasticsearch/reference/current/search-aggregations-bucket-datehistogram-aggregation.html#calendar_intervals
//
// We will parse it to Solr as much as possible
func convertCalendarInterval() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		if query.CalendarInterval == nil {
			defaultCalenderInterval := "1M"
			query.CalendarInterval = &defaultCalenderInterval
		}

		calenderIntervalMap := map[string]string{
			"1m": "+1MINUTE",
			"1h": "+1HOUR",
			"1d": "+1DAY",
			"1w": "+1WEEK",
			"1M": "+1MONTH",
			"1q": "+3MONTH",
			"1y": "+1YEAR",
		}

		solrEquivalent, ok := calenderIntervalMap[*query.CalendarInterval]
		if !ok {
			return "", "+1MONTH", nil
		}

		return "", solrEquivalent, nil
	}
}

// convertDeepPagination will convert the value of deepPagination
// to its Solr equivalent.
//
// deepPagination will in a way map to `cursorMark`
//
// This function will take care of parsing deepPaginationConfig as well
func convertDeepPagination() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		// DeepPagination can only be used if the `sortField` value is passed.
		// By default the sortField will be set to `score` and it is not considered a valid
		// field to use cursorMark on so a custom sortField is required for cursorMark to work.

		if query.SortField == nil || *query.SortField == "" || *query.SortField == "score" || query.DeepPagination == nil || !*query.DeepPagination {
			return "", "", nil
		}

		cursorValueToUse := "*"

		// If deepPagination is enabled and cursor is also passed,
		// set the passed value of cursor as the one to use.
		if query.DeepPagination != nil &&
			*query.DeepPagination &&
			query.DeepPaginationConfig != nil &&
			query.DeepPaginationConfig.Cursor != nil &&
			*query.DeepPaginationConfig.Cursor != "" {
			// Set the cursorMark and return
			cursorValueToUse = *query.DeepPaginationConfig.Cursor
		}

		// Finally return the mapping for the cursor in Solr
		return "cursorMark", cursorValueToUse, nil
	}
}

// convertIncludeValues will convert the includeValues field to its
// Solr equivalent.
//
// includeValues will map to facet.contains
func convertIncludeValues() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		if query.Type != querytranslate.Term || query.IncludeValues == nil {
			return "", "", nil
		}

		if len(*query.IncludeValues) < 1 {
			return "", "", nil
		}

		return "facet.contains", strings.Join(*query.IncludeValues, "<-->"), nil
	}
}

// convertExcludeValues will convert the excludeValues field to its
// Solr equivalent.
//
// includeValues will map to facet.excludeTerms
func convertExcludeValues() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		if query.Type != querytranslate.Term || query.ExcludeValues == nil {
			return "", "", nil
		}

		if len(*query.ExcludeValues) < 1 {
			return "", "", nil
		}

		return "facet.excludeTerms", strings.Join(*query.ExcludeValues, "<-->"), nil
	}
}

// convertReactField will convert the `react` field to its
// Solr equivalent.
//
// react will map to `fq` in Solr.
//
// This function will not handle the case of react field
// not being present. Should be handled in the parent method.
func convertReactField() ConvertFunc {
	return func(query *querytranslate.Query, allQueries *[]querytranslate.Query) (string, string, *Error) {
		// The `react` field can support being one of the following:
		// - map of string to array of strings -> based on passed operator
		// - array of strings ->
		// - string -> NOT
		react, reactErr := evalReactToSolr(*query.React, "", allQueries, query)
		if reactErr != nil {
			log.Warnln(logTag, ": ", reactErr.Err.Error())
			return "", "", reactErr
		}

		if react == "()" {
			return "", "", nil
		}

		log.Debug(logTag, ": fq: ", pretty.Formatter(react))
		return "fq", react, nil
	}
}

// evalReactToSolr will evaluate the react prop for Solr.
//
// Conjunction used by default is AND. This is for the case of
// array of strings.
func evalReactToSolr(react interface{}, conjunction string, allQueries *[]querytranslate.Query, originalQuery *querytranslate.Query) (string, *Error) {
	// Set default conjunction to AND
	if conjunction == "" {
		conjunction = "AND"
	}

	if allQueries == nil {
		return "", &Error{
			Err:  fmt.Errorf("invalid query list passed"),
			Code: http.StatusInternalServerError,
		}
	}

	reactAsMap, asMapOk := react.(map[string]interface{})

	if asMapOk {
		topFqArr := make([]string, 0)

		if reactAsMap["and"] != nil {
			builtFq, err := evalReactToSolr(reactAsMap["and"], "AND", allQueries, originalQuery)
			if err != nil {
				return "", err
			}

			topFqArr = append(topFqArr, builtFq)
		}

		if reactAsMap["or"] != nil {
			builtFq, err := evalReactToSolr(reactAsMap["or"], "OR", allQueries, originalQuery)
			if err != nil {
				return "", err
			}

			topFqArr = append(topFqArr, builtFq)
		}

		// Check on adding support for `not
		if reactAsMap["not"] != nil {
			builtFq, err := evalReactToSolr(reactAsMap["not"], "NOT", allQueries, originalQuery)
			if err != nil {
				return "", err
			}

			topFqArr = append(topFqArr, builtFq)
		}

		// If there's just one item in the topFqArr, we return it as is.
		// else we will have to join it with an operator.
		if len(topFqArr) == 1 {
			return topFqArr[0], nil
		}

		// TODO: Check if below should be `OR` or something else
		return fmt.Sprintf("(%s)", strings.Join(topFqArr, " OR ")), nil

		// NOTE: Make sure return here since the below code should be executed
		// only if the above fails.
	}

	reactAsArray, asArrOk := react.([]interface{})
	if asArrOk {
		// Send every array to the same function and
		// save the result using
		fqArr := make([]string, 0)

		for _, react := range reactAsArray {
			fqEach, err := evalReactToSolr(react, conjunction, allQueries, originalQuery)
			if err != nil {
				return "", err
			}

			// In case id is invalid, don't add it.
			if fqEach == "" {
				continue
			}

			fqEach = fmt.Sprintf("(%s)", fqEach)

			fqArr = append(fqArr, fqEach)
		}

		// Once all the fq's are built, we need to connect them
		// with the passed conjunction.
		joinedWithConjunction := strings.Join(fqArr, fmt.Sprintf(" %s ", conjunction))

		// if conjunction is NOT, we need to add a prefix NOT
		if conjunction == "NOT" {
			joinedWithConjunction = fmt.Sprintf("NOT %s", joinedWithConjunction)
		}

		return fmt.Sprintf("(%s)", joinedWithConjunction), nil

		// NOTE: Make sure return here since the below code should be executed
		// only if the above fails.
	}

	reactAsString, asStrOk := react.(string)
	if !asStrOk {
		errMsg := "Invalid value passed for `react`"
		log.Warnln(logTag, ": ", errMsg)
		return "", &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}

	}

	// Parse the string ID, extract the dataField and value
	// and return it.
	var matchedQuery *querytranslate.Query
	for _, query := range *allQueries {
		if query.ID != nil && reactAsString == *query.ID {
			matchedQuery = &query
			break
		}
	}

	if matchedQuery == nil {
		// Throw error indicating invalid query ID
		// referenced
		errMsg := fmt.Sprintf("couldn't find any query with the ID: `%s` as referenced in `react`", reactAsString)
		log.Warnln(logTag, ": ", errMsg)

		// Ignore if the `id` is invalid, this is a requirement from the frontend.
		return "", nil
	}

	// If customQuery is passed, then try to extract and use
	// it.
	if matchedQuery.CustomQuery != nil {
		parsedQueryMap, parseErr := parsePassedQuery(matchedQuery.CustomQuery)

		if parseErr != nil {
			// Throw error
			return "", parseErr
		}

		if len(parsedQueryMap) >= 1 {
			// Try to see if `qf` is present in the query
			fqValue, fqOk := parsedQueryMap["fq"]

			if !fqOk {
				return "", &Error{
					Err:  fmt.Errorf("`fq` not passed in customQuery, cannot use it for react"),
					Code: http.StatusBadRequest,
				}
			}

			return fqValue, nil
		}
	}

	// If type is range and value is
	if matchedQuery.Type == querytranslate.Range || matchedQuery.Type == querytranslate.Term {
		if matchedQuery.Value == nil {
			return "", nil
		}

		// Try to parse the value into an array and check if it is empty.
		queryAsArr, asArrOk := (*matchedQuery.Value).([]interface{})
		if asArrOk {
			if len(queryAsArr) <= 0 {
				return "", nil
			}
		}
	}

	// Determine the query value
	//
	// Set the value as * if not present
	if matchedQuery.Value == nil {
		defaultValue := new(interface{})
		*defaultValue = "*"
		matchedQuery.Value = defaultValue
	}

	if matchedQuery.Value != nil && *matchedQuery.Value == "" {
		var defaultValue interface{} = "*"
		matchedQuery.Value = &defaultValue
	}
	queryField, queryValue, queryValueErr := convertValue()(matchedQuery, nil)
	if queryValueErr != nil {
		errMsg := fmt.Sprintf("error while extracting the query value from the matched query with ID: %s with err: %s", *matchedQuery.ID, queryValueErr.Err.Error())
		log.Warnln(logTag, ": ", errMsg)
		return "", &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Build the q value since queryFormat modifies it.
	// This will modify q only if the type is of suggestion or search
	if matchedQuery.QueryFormat != nil {
		_, solrValueVal, conversionErr := convertQueryFormat()(matchedQuery, nil)
		if conversionErr != nil {
			return "", conversionErr
		}

		if solrValueVal != "" {
			queryValue = solrValueVal
		}
	}

	// Sometimes convertValue will return `fq` value so in such
	// a case we need to remove the dataField from the query since
	// we only want the value in react.
	if queryField == "fq" {
		valueSplitted := strings.Split(queryValue, ":")
		if len(valueSplitted) > 1 {
			valueSplitted = valueSplitted[1:]
			queryValue = strings.Join(valueSplitted, "")
		}
	}

	// Extract the dataField values for the selected query.
	// We cannot support field weights in the dataField.
	_, dfAsMapOk := matchedQuery.DataField.(map[string]interface{})
	if dfAsMapOk {
		// Cannot accept dataField with weights.
		// Raise error
		errMsg := fmt.Sprintf("cannot accept dataField with weights and work with `react` for query with ID: %s", *matchedQuery.ID)
		log.Warnln(logTag, ": ", errMsg)
		return "", &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	dfAsArr, dfAsArrOk := matchedQuery.DataField.([]interface{})
	if dfAsArrOk {
		// Parse the array of interface into array of strings
		dfAsStrArr := make([]string, 0)

		for dfPosition, df := range dfAsArr {
			dfAsStr, dfAsStrOk := df.(string)
			if !dfAsStrOk {
				errMsg := fmt.Sprintf("invalid dataField value passed in position %d for query with ID: %s", dfPosition, *matchedQuery.ID)
				return "", &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusBadRequest,
				}
			}

			dfAsStrArr = append(dfAsStrArr, dfAsStr)
		}

		if matchedQuery.Type != querytranslate.Term {
			if queryValue != "*" && queryValue != "" {
				queryValue = fmt.Sprintf("\"%s\"", queryValue)
			}
			return fmt.Sprintf("(%s:%s)", strings.Join(dfAsStrArr, ","), queryValue), nil
		}

		// If type is term, we need to handle it in a different way.
		//
		// The idea is to support pivot faceting if the query being reacted to
		// contains more than one dataField.
		//
		// The queryValue is joined based on the queryFormat and it defaults out to `OR`.
		// We will try to split the value on the basis of OR first and then try AND, one of them
		// should work
		defaultQF := "OR"
		if matchedQuery.QueryFormat == nil {
			matchedQuery.QueryFormat = &defaultQF
		}

		upperedQueryFormat := strings.ToUpper(*matchedQuery.QueryFormat)

		// Above value should be a supported value since otherwise an error would've been
		// raised and the execution would not have reached this far.

		valueSplitted := strings.Split(queryValue, fmt.Sprintf(" %s ", upperedQueryFormat))

		valueMappedToDf := make([]string, 0)

		for _, valueEach := range valueSplitted {
			// We need to split the value by `>` and then map each layer
			// to the dataFields layer. If the layer is not present, we need
			// to skip that.
			splitRe := regexp.MustCompile(" ?> ?")
			valueEachSplitted := splitRe.Split(valueEach, -1)

			// Make sure length of splitted is not more than length
			// of dataField array, else slice it.
			if len(valueEachSplitted) > len(dfAsStrArr) {
				valueEachSplitted = valueEachSplitted[:len(dfAsStrArr)]
			}

			// Iterate the valueEachSplitted and map it to df according to index.
			valueEachMap := make([]string, 0)
			for valueSplittedIndex, valueSplitted := range valueEachSplitted {
				dfForIndex := dfAsStrArr[valueSplittedIndex]
				if valueSplitted != "*" && valueSplitted != "" {
					valueSplitted = fmt.Sprintf("\"%s\"", valueSplitted)
				}
				valueEachMap = append(valueEachMap, fmt.Sprintf("(%s:%s)", dfForIndex, valueSplitted))
			}

			valueMappedToDf = append(valueMappedToDf, fmt.Sprint("(", strings.Join(valueEachMap, fmt.Sprintf(" %s ", "AND")), ")"))
		}

		// Join the valueMappedToDf based on the conjunction and return it.
		return strings.Join(valueMappedToDf, fmt.Sprintf(" %s ", upperedQueryFormat)), nil
	}

	// Try to extract the dataField as string
	dfAsStr, dfAsStrOk := matchedQuery.DataField.(string)
	if !dfAsStrOk {
		// DataField is invalid
		errMsg := fmt.Sprintf("passed `dataField` is invalid for query with ID: %s", *matchedQuery.ID)
		log.Warnln(logTag, ": ", errMsg)

		// At this point, we will have to ignore this field
		// since we cannot match to all fields on Solr.
		//
		// If for this query, dataField is not available, set the `q` value
		// of the master query as the value of this query.
		log.Debug(logTag, ": matchedQuery.Value", pretty.Formatter(matchedQuery.Value))
		if matchedQuery.Value != nil && originalQuery.Type != querytranslate.Geo {
			originalQuery.Value = matchedQuery.Value
		}
		return "", nil
	}

	if queryValue != "*" && queryValue != "" {
		queryValue = fmt.Sprintf("\"%s\"", queryValue)
	}
	return fmt.Sprintf("(%s:%s)", dfAsStr, queryValue), nil
}

// parsePassedQuery will parse the passed query from fields
// like defaultQuery or customQuery and return it
// in a format that is usable directly.
//
// Errors will also be handled in this functions.
func parsePassedQuery(query *map[string]interface{}) (map[string]string, *Error) {
	queryMap := make(map[string]string)

	// Check if passed query is nil or not
	// If it is, then just ignore
	if query == nil {
		return queryMap, nil
	}

	// Parse only if `query` is passed
	queryPassed, isQueryPassed := (*query)["query"]

	if !isQueryPassed {
		// Not an error, just ignore.
		return queryMap, nil
	}

	queryAsString, queryOk := queryPassed.(string)
	if !queryOk {
		return queryMap, &Error{
			Err:  errors.New("only `string` is supported in `defaultQuery.query` for Solr."),
			Code: http.StatusBadRequest,
		}
	}

	// If the first element of the string is not a `?`, make it
	// one.
	if string(queryAsString[0]) != "?" {
		queryAsString = "?" + queryAsString
	}

	// Decode the URL
	decodedURL, decodeErr := url.PathUnescape(queryAsString)
	if decodeErr != nil {
		return queryMap, &Error{
			Err:  fmt.Errorf("error while decoding URL: %s", decodeErr.Error()),
			Code: http.StatusInternalServerError,
		}
	}

	parsedURL, parseErr := url.Parse(decodedURL)
	if parseErr != nil {
		return queryMap, &Error{
			Err:  fmt.Errorf("error while parsing URL: %s", parseErr.Error()),
			Code: http.StatusInternalServerError,
		}
	}

	parsedMap, mapParseErr := url.ParseQuery(parsedURL.RawQuery)
	if mapParseErr != nil {
		return queryMap, &Error{
			Err:  fmt.Errorf("error while parsing query into a map: %s", mapParseErr.Error()),
			Code: http.StatusInternalServerError,
		}
	}

	for key, value := range parsedMap {
		queryMap[key] = strings.Join(value, " ")
	}

	return queryMap, nil
}

// getMinMaxValue will get the min and max value for the passed
// field by using a stats query.
func getMinMaxValue(field string, urlToHit string, headers *map[string]string) (int, int, error) {
	query := fmt.Sprintf("?q=*:*&stats=true&stats.field=%s", field)
	url := urlToHit + query

	// Set the headers in the request
	headerToSend := make(http.Header)
	for key, value := range *headers {
		headerToSend.Add(key, value)
	}

	responseInBytes, _, err := util.MakeRequestWithHeader(url, "GET", []byte(""), headerToSend)
	if err != nil {
		return 0, 0, err
	}

	// Parse the response into a map
	responseAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(responseInBytes, &responseAsMap)
	if unmarshalErr != nil {
		return 0, 0, unmarshalErr
	}

	log.Debug(logTag, ": min max response: ", pretty.Formatter(responseAsMap))
	statsAsMap, statOk := responseAsMap["stats"].(map[string]interface{})
	if !statOk {
		return 0, 0, fmt.Errorf("error while parsing the stats to get min max")
	}

	statsFieldsAsMap, statFieldOk := statsAsMap["stats_fields"].(map[string]interface{})
	if !statFieldOk {
		return 0, 0, fmt.Errorf("error while parsing the stats_fields to get min max value")
	}

	fieldAsMap, fieldOk := statsFieldsAsMap[field].(map[string]interface{})
	if !fieldOk {
		return 0, 0, fmt.Errorf("error while getting the field from the stats to get min max value")
	}

	minValue, minOk := fieldAsMap["min"]
	if !minOk {
		return 0, 0, fmt.Errorf("error while parsing the min value for field")
	}

	maxValue, maxOk := fieldAsMap["max"]
	if !maxOk {
		return 0, 0, fmt.Errorf("error while parsing the max value for field")
	}

	minAsFloat := minValue.(float64)
	maxAsFloat := maxValue.(float64)

	return int(minAsFloat), int(maxAsFloat), nil
}
