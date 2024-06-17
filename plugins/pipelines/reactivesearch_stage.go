package pipelines

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	"github.com/appbaseio-confidential/reactivesearch/plugins/suggestions"
	log "github.com/sirupsen/logrus"
)

type ReactiveSearchQueryInput struct {
	Backend querytranslate.Backend `json:"backend,omitempty" jsonschema:"title=Search Backend" jsonschema_description:"Search backend, defaults to 'elasticsearch'."`
}

func executeReactivesearchStage(
	stage ESPipelineStage,
	stageInputs *string,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time,
	hasBoostStage bool) ([]byte, bool, *Error) {

	id := getStageID(stage)

	scriptContextInBytes := globalScriptContext.Get()

	var rsAPIBody querytranslate.RSQuery
	var scriptContext rules.ScriptContext
	err := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	err2 := json.Unmarshal([]byte(scriptContext.Request.Body), &rsAPIBody)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}
	// Apply preferences as per searchbox Id
	suggestions.ApplySuggestionsPreferences(rsAPIBody, nil)
	// Update rsAPIBody
	rsAPIRequest.Put(&rsAPIBody)

	// Store the original RS API body in context since it will
	// be modified in this stage.
	scriptContext.Environments["ORIGINAL_RS_BODY"] = scriptContext.Request.Body

	var inputs ReactiveSearchQueryInput
	if stageInputs != nil {
		err2 := json.Unmarshal([]byte(*stageInputs), &inputs)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			return nil, false, &Error{
				Err: err2,
			}
		}
	}
	var query string
	var esIndependentRequests string
	var mlQSMap string
	if inputs.Backend == querytranslate.MongoDB {
		// translate mongodb query
		script := `function handleRequest() {
			const requestBody = JSON.parse(context.request.body);
			const a = new reactivesearch.ReactiveSearch({})
			return a.translate(requestBody.query);
		}`
		scriptOutput, _, err := rules.RunScript(globalScriptContext.Get(), script, 5*time.Second)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, false, &Error{
				Err: err,
			}
		}

		// There is a possibility that even if the `err` is reported as `nil`,
		// the scriptOutput might contain an error. This error might be something
		// that's handled in the script for some kind of user error like the request
		// body is not acceptable or something like that.
		//
		// In order to check that, we will need to unmarshal the response into a map
		// (if possible) and then check if it contains a key `error` in it.
		//
		// Most probably the map will be of the structure:
		// {
		//   "error": "",
		//   "code": 400
		// }
		//
		// If code is not present, we will default to a 500 else it will be as passed
		// in the response.
		outputAsMap := make(map[string]interface{})
		unmarshalErr := json.Unmarshal(scriptOutput, &outputAsMap)

		if unmarshalErr != nil {
			// This doesn't mean that the response is not the expected one since
			// a map[string][]map[string]interface{} will also unmarshal into
			// the above structure.
			//
			// This is probably some other issue that we should report.
			errMsg := fmt.Errorf("error while unmarshalling `reactivesearchQuery` response into map to check for errors: %s", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err:  errMsg,
				Code: http.StatusInternalServerError,
			}
		}

		// Check if `error` key is present in the top level
		isErrPresent := false
		for key := range outputAsMap {
			if key == "error" {
				isErrPresent = true
				break
			}
		}

		// If `error` key is present, we need to report the error
		if isErrPresent {
			// The `error` will be mapped to an object from where we will
			// need to read the `error` key and the `code` key.
			//
			// The `error` key will be an array where the 1'th element will be a string
			// with the error message.
			//
			// `code` will be an integer.
			errAsMap, asMapOk := outputAsMap["error"].(map[string]interface{})
			if !asMapOk {
				// We want to report the error even though we don't know how
				// to extract the error from MongoDB output.
				return nil, false, &Error{
					Err:  fmt.Errorf("unknown error for `reactivesearchQuery` conversion"),
					Code: http.StatusInternalServerError,
				}
			}

			// If it is a map, try ot parse the actual error and code.
			isNestedErrPresent := false
			isNestedCodePresent := false
			errCode := http.StatusInternalServerError
			errMsg := "error while executing `reactivesearchQuery`: unknown error"

			for key := range errAsMap {
				if key == "error" {
					isNestedErrPresent = true
				}

				if key == "code" {
					isNestedCodePresent = true
				}

				// Break the loop if we have already determined that both
				// code and error are present.
				if isNestedErrPresent && isNestedCodePresent {
					break
				}
			}

			// If `code` is present, parse it
			if isNestedCodePresent {
				// Parse the `code` value into a float64 and the convert it
				// into an integer.
				codeAsFloat, asFloatOk := errAsMap["code"].(float64)
				if asFloatOk {
					codeAsInt := int(codeAsFloat)
					errCode = codeAsInt
				}
			}

			// If `error` is present, parse it accordingly.
			if isNestedErrPresent {
				errAsArr, asArrOk := errAsMap["error"].([]interface{})
				if asArrOk {
					// Find the first valid error in the array, convert it to
					// string and accordingly use it.
					for errIndex, errEach := range errAsArr {
						if errEach == nil {
							continue
						}

						errAsStr, asStrOk := errEach.(string)
						if asStrOk {
							errMsg = fmt.Sprintf("error for query at index `%d` with: %s", errIndex, errAsStr)
							break
						}
					}
				} else {
					// Try to parse as string if possible, though it's very unlikely
					errAsStr, asStrOk := errAsMap["error"].(string)
					if asStrOk {
						errMsg = errAsStr
					}
				}
			}

			return nil, false, &Error{
				Err:  fmt.Errorf("%s", errMsg),
				Code: errCode,
			}
		}

		query = string(scriptOutput)
	} else if inputs.Backend == querytranslate.Solr {
		// Validate the RS body values since we do not support
		// all keys conversion to Solr
		validateErr := ValidateRSToSolrKey(&rsAPIBody.Query)
		if validateErr != nil {
			log.Warnln(logTag, ": validation failed for RS to Solr conversion: ", validateErr.Err)
			return nil, false, validateErr
		}

		// Validate the RS fields
		for _, query := range rsAPIBody.Query {
			if query.EnableEndpointSuggestions == nil || *query.EnableEndpointSuggestions {
				if query.Endpoint != nil && (query.Endpoint.URL == nil || *query.Endpoint.URL == "") {
					return nil, false, &Error{
						Err:  errors.New("`endpoint.url` is a required property when `endpoint` is passed. Remove the `endpoint` property if it's not used."),
						Code: http.StatusBadRequest,
					}

					// Setting the default method etc will be done during
					// sending the independent queries and not in this part of the code.
				}
			}
		}

		// Build every query and save it in the request body
		builtQueryMap := make(map[string]interface{})
		for _, query := range rsAPIBody.Query {
			// If query.execute is disabled then don't build the query.
			if query.Execute != nil && !*query.Execute {
				continue
			}

			// Marshal the query that was converted
			queryInBytes, marshalErr := json.Marshal(query)
			if marshalErr != nil {
				log.Errorln(logTag, ": error while marshalling the translated query to store it, ", marshalErr)
				return nil, false, &Error{
					Err:  marshalErr,
					Code: http.StatusInternalServerError,
				}
			}

			// Depending on the type of query, i:e endpoint based on direct
			// we will build each query and then solr stage will take care of
			// executing it.

			if query.Endpoint != nil {
				// Build an execution query instead of Solr query.
				queryAsMap, queryBuildErr := querytranslate.BuildIndependentRequest(query, rsAPIBody)
				if queryBuildErr != nil {
					log.Warnln(logTag, ": ", queryBuildErr)
					return nil, false, &Error{
						Err:  queryBuildErr,
						Code: http.StatusBadRequest,
					}
				}

				// Add some special flags to the map
				queryAsMap["_isEndpoint"] = "true"
				queryAsMap["_original"] = string(queryInBytes)

				builtQueryMap[*query.ID] = queryAsMap

				// Don't continue execution since it is endpoint type.
				continue
			}

			builtQuery, translateErr := TranslateToSolr(query, &rsAPIBody.Query)
			if translateErr != nil {
				log.Warnln(logTag, ": error while translating RS query to Solr query, ", translateErr)
				return nil, false, translateErr
			}

			// Add the original query string as a special key
			// in the builtQuery
			builtQuery["_original"] = string(queryInBytes)

			// Else assign the built query to the query map.
			builtQueryMap[*query.ID] = builtQuery
		}

		// Stringify the map now
		queryAsString, queryErr := json.Marshal(builtQueryMap)
		if queryErr != nil {
			log.Errorln(logTag, ": error while marshalling the built query map, ", queryErr)
			return nil, false, &Error{
				Err:  queryErr,
				Code: http.StatusInternalServerError,
			}
		}

		query = string(queryAsString)
	} else if inputs.Backend == querytranslate.Zinc {
		// Parse the index from the envs and throw an error
		// if it is not present.
		index, indexPresent := scriptEnvs["index"]
		if !indexPresent {
			errMsg := "`index` should be present as an env for zinc stage"
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusBadRequest,
			}
		}

		indexAsString := ""

		// `index` can be both a string and an array, so we need to
		// support both.
		indexAsArr, asArrOk := index.([]interface{})
		if !asArrOk {
			// Parse as a string else throw error
			indexStr, asStrOk := index.(string)
			if !asStrOk {
				errMsg := "`index` is neither array nor string, should be one of the two"
				log.Warnln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusBadRequest,
				}
			}

			indexAsString = indexStr
		} else {
			// Throw error if array is empty
			if len(indexAsArr) < 1 {
				errMsg := "`index` array should contain at-least 1 element"
				log.Warnln(logTag, ": ", errMsg)
				return nil, false, &Error{
					Err:  fmt.Errorf(errMsg),
					Code: http.StatusBadRequest,
				}
			}

			// Parse the first element of `index` to string and use as
			// the index.
			for _, index := range indexAsArr {
				indexStr, asStrOk := index.(string)
				if !asStrOk {
					errMsg := "element at 0th index for `index` is non-string value"
					log.Warnln(logTag, ": ", errMsg)
					return nil, false, &Error{
						Err:  fmt.Errorf(errMsg),
						Code: http.StatusBadRequest,
					}
				}

				indexAsString = indexStr
			}
		}

		if indexAsString == "" {
			errMsg := "couldn't parse a value of `index`"
			log.Warnln(logTag, ": ", errMsg)
			return nil, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusBadRequest,
			}
		}

		// Translate to zinc
		zincQuery, zincTranslateErr := TranslateToZinc(&rsAPIBody, indexAsString)
		if zincTranslateErr != nil {
			log.Warnln(logTag, ": error while translating to zinc: ", zincTranslateErr.Err.Error())
			return nil, false, zincTranslateErr
		}

		query = zincQuery
	} else {
		// extract ip from envs
		var ip string
		if ipv4, ok := scriptEnvs["ipv4"]; ok {
			ipAsString, ok := ipv4.(string)
			if ok {
				ip = ipAsString
			}
		}
		if ip == "" {
			if ipv6, ok := scriptEnvs["ipv6"]; ok {
				ipAsString, ok := ipv6.(string)
				if ok {
					ip = ipAsString
				}
			}
		}
		rsAPIBodyToTranslate := rsAPIBody

		rsAPIBodyToTranslate.Query = make([]querytranslate.Query, len(rsAPIBody.Query))
		copy(rsAPIBodyToTranslate.Query, rsAPIBody.Query)

		if hasBoostStage {
			boostQueryIndex := getBoostQueryIndex(&rsAPIBodyToTranslate)
			if boostQueryIndex != -1 {
				query := rsAPIBodyToTranslate.Query[boostQueryIndex]
				// if from value is set then update the size value to `from + size`
				size := getComputedSize(query)
				if size != nil {
					rsAPIBodyToTranslate.Query[boostQueryIndex].Size = size
				}
			}
		}
		var preference *string
		paramsAsMap, ok := scriptEnvs["urlValues"].(map[string]interface{})
		if ok {
			for k, v := range paramsAsMap {
				if k == "preference" {
					paramValue, ok := v.(string)
					if ok {
						preference = &paramValue
					}
				}
			}
		}

		esQuery, _, _, err := querytranslate.TranslateQuery(rsAPIBodyToTranslate, ip, nil, preference)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, false, &Error{
				Err: err,
			}
		}

		query = esQuery
	}

	independentRequests, requestBuildErr := querytranslate.BuildIndependentRequests(rsAPIBody)
	if requestBuildErr != nil {
		log.Errorln(logTag, ": ", err)
		return nil, false, &Error{
			Err: err,
		}
	}

	// Marshal the response for independent requests
	independentRequestAsString, marshalErr := json.Marshal(independentRequests)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshalling independent requests built: ", marshalErr)
		return nil, false, &Error{
			Err: marshalErr,
		}
	}
	esIndependentRequests = string(independentRequestAsString)

	independentRequestID := fmt.Sprintf("%s_INDEPENDENT_REQUESTS", *id)
	backendID := fmt.Sprintf("%s_RS_BACKEND", inputs.Backend)
	mlQSKey := "ML_QS_PARAMS"

	var output interface{}
	// Update script context
	if async {
		// write output to a top-level variable
		output = map[string]interface{}{
			*id:                                query,
			independentRequestID:               esIndependentRequests,
			backendID:                          inputs.Backend.String(),
			fmt.Sprintf("%s_%s", *id, mlQSKey): mlQSMap,
		}
	} else {
		scriptContext.Request.Body = query
		scriptContext.Environments["INDEPENDENT_REQUESTS"] = esIndependentRequests
		scriptContext.Environments[mlQSKey] = mlQSMap
		scriptContext.Environments["RS_BACKEND"] = inputs.Backend.String()
		output = scriptContext
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
