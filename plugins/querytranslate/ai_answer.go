package querytranslate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/appbaseio/reactivesearch-api/plugins/openai"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/buger/jsonparser"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
)

const (
	OpenAIAPIURL         string = "https://api.openai.com/v1"
	ModelUsed            string = "gpt-3.5-turbo"
	KeyToInject          string = "AIAnswer"
	SessionIDKeyToInject string = "AISessionId"
)

// ShouldExecuteAIAnswer will check if the passed query
// is eligible for executing AIAnswer and will accordingly
// return the result.
func ShouldExecuteAIAnswer(query Query) bool {
	return query.EnableAI != nil && *query.EnableAI
}

// ExecuteAIAnswerInQuery will parse the RS Query passed, find out
// the queries that need to be executed for AIAnswer and then
// inject the responses accordingly.
func ExecuteAIAnswerInQuery(rsRequest *RSQuery, transformedResponse []byte, indices []string, sessionMap *openai.SessionIdToChatGPTResponse, userId string) ([]byte, error) {
	openAIInstance := openai.Instance()

	// Check whether or not all the indexes in the list are
	// whitelisted for allowing OpenAI
	areAllIndexesWhitelisted := true
	for _, index := range indices {
		if !openAIInstance.GetConfig().IsIndexWhitelisted(index) {
			areAllIndexesWhitelisted = false
			continue
		}
	}

	// Determine what userId should be used.
	//
	// If the userId is passed in settings, we use that else fallback
	// to the IP address
	if rsRequest.Settings != nil && rsRequest.Settings.UserID != nil && strings.TrimSpace(*rsRequest.Settings.UserID) != "" {
		userId = *rsRequest.Settings.UserID
	}

	// Figure out the queries that need to be executed for AIAnswer
	for _, query := range (*rsRequest).Query {
		if !ShouldExecuteAIAnswer(query) {
			continue
		}

		queryIDEach := *query.ID

		// Try to extract the value from `react` if `value` is not present
		// and accordingly throw an error
		queryValue, valueFetchErr := FindValueForAI(query, *rsRequest)
		if valueFetchErr != nil {
			errMsg := fmt.Sprintf("error while extracting value to use for AIAnswer: %s", valueFetchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			updatedResponse, setErr := jsonparser.Set(transformedResponse, []byte(`"`+errMsg+`"`), queryIDEach, KeyToInject, "error")
			if setErr != nil {
				log.Errorln(logTag, ": ", "error while injecting OpenAI error in response")
			} else {
				transformedResponse = updatedResponse
			}

			// Skip execution
			continue
		}
		log.Debug(logTag, ": value found is ", queryValue)

		// Make sure OpenAI is allowed for this plan, if not allowed then inject an error
		// message in that key instead of throwing an error
		if !openai.IsOpenAIAllowed() {
			errMsg := "Plan does not have access to use OpenAI feature. Please contact support@reactivesearch.io for enabling it or upgrade to a production plan."
			log.Warnln(logTag, ": ", errMsg)
			updatedResponse, setErr := jsonparser.Set(transformedResponse, []byte(`"`+errMsg+`"`), queryIDEach, KeyToInject, "error")
			if setErr != nil {
				log.Errorln(logTag, ": ", "error while injecting OpenAI error in response")
			} else {
				transformedResponse = updatedResponse
			}

			// Skip execution
			continue
		}

		// Make sure OpenAI is enabled
		if !openAIInstance.IsOpenAIEnabled() {
			errMsg := "OpenAI is disabled, enable it by the API endpoint or by visiting dashboard: https://dash.reactivesearch.io"
			log.Warnln(logTag, ": ", errMsg)
			updatedResponse, setErr := jsonparser.Set(transformedResponse, []byte(`"`+errMsg+`"`), queryIDEach, KeyToInject, "error")
			if setErr != nil {
				log.Errorln(logTag, ": ", "error while injecting OpenAI error in response")
			} else {
				transformedResponse = updatedResponse
			}

			continue
		}

		// Make sure that the API Key is valid and not empty
		if !openAIInstance.GetConfig().IsKeyValid() {
			errMsg := "OpenAI API Key should be valid. Please contact support@reactivesearch.io for setting up your OpenAI API Key."
			log.Warnln(logTag, ": ", errMsg)
			updatedResponse, setErr := jsonparser.Set(transformedResponse, []byte(`"`+errMsg+`"`), queryIDEach, KeyToInject, "error")
			if setErr != nil {
				log.Errorln(logTag, ": ", "error while injecting OpenAI error in response")
			} else {
				transformedResponse = updatedResponse
			}

			// Skip execution
			continue
		}

		// Extract the index, if any from the query
		indexFromQuery := query.Index

		// If the index is present, we need to check if it is whitelisted and accordingly
		// execute.
		//
		// If it is not present and the overall indices passed are also not all whitelisted
		// then we will throw an error
		if indexFromQuery != nil && *indexFromQuery != "" {
			// The query based index is present
			if !openAIInstance.GetConfig().IsIndexWhitelisted(*indexFromQuery) {
				// The query based index is not whitelisted, we need to throw an error
				errMsg := fmt.Sprintf("`%s` is not whitelisted for OpenAI execution", *indexFromQuery)
				log.Warnln(logTag, ": ", errMsg)
				return transformedResponse, fmt.Errorf(errMsg)
			}
		} else if !areAllIndexesWhitelisted {
			// One or more of the passed indices are not allowed for OpenAI execution
			errMsg := "one or more of the passed indices are not allowed for OpenAI execution"
			log.Warnln(logTag, ": ", errMsg)
			return transformedResponse, fmt.Errorf(errMsg)
		}

		// Check API type and accordingly extract the values
		if (openAIInstance.GetConfig().GetAPIType() == openai.AzureType) &&
			(strings.TrimSpace(openAIInstance.GetConfig().GetAzureURL()) == "" ||
				strings.TrimSpace(openAIInstance.GetConfig().GetAzureVersion()) == "") {
			errMsg := "Azure Base URL and Version are required values when API type is set to `azure`. Either add the values or change API type to `openai`."
			log.Warnln(logTag, ": ", errMsg)
			updatedResponse, setErr := jsonparser.Set(transformedResponse, []byte(`"`+errMsg+`"`), queryIDEach, KeyToInject, "error")
			if setErr != nil {
				log.Errorln(logTag, ": ", "error while injecting OpenAI error in response")
			} else {
				transformedResponse = updatedResponse
			}

			// Skip execution
			continue
		}

		var AIConfigUsed AIConfig = AIConfig{}

		// Validate the AIConfig body
		if query.AIConfig != nil {
			AIConfigUsed = *query.AIConfig
		}

		// Validate the temperature value if passed by the user
		if AIConfigUsed.Temperature != nil {
			// Make sure the temperature value is between 0 and 2
			if *AIConfigUsed.Temperature < 0 || *AIConfigUsed.Temperature > 2 {
				errMsg := "`temperature` should be in the range of 0 and 2"
				log.Warnln(logTag, ": ", errMsg)
				return transformedResponse, fmt.Errorf(errMsg)
			}
		}

		// Set default values
		if AIConfigUsed.SystemPrompt == nil {
			promptToSet := openAIInstance.GetConfig().GetSystemPrompt()
			AIConfigUsed.SystemPrompt = &promptToSet
		}

		if AIConfigUsed.QueryTemplate == nil {
			defaultQueryTemplate := "Answer the query: '${value}'. Think step-by-step, cite the source after the answer and ensure the source is from the provided context."
			AIConfigUsed.QueryTemplate = &defaultQueryTemplate
		}

		if AIConfigUsed.TopDocsForContext == nil {
			defaultTopDocsForContext := 3
			AIConfigUsed.TopDocsForContext = &defaultTopDocsForContext
		}

		if AIConfigUsed.StrictSelection == nil {
			defaultStrictSelection := false
			AIConfigUsed.StrictSelection = &defaultStrictSelection
		}

		if AIConfigUsed.MaxTokens == nil {
			tokensSet := openAIInstance.GetConfig().GetMaxTokens()
			AIConfigUsed.MaxTokens = &tokensSet
		}

		if AIConfigUsed.MinTokens == nil {
			tokensSet := openAIInstance.GetConfig().GetMinTokens()
			AIConfigUsed.MinTokens = &tokensSet
		}

		// Make sure there is no error from ES, if there is then we cannot continue.
		//
		// We will check this by checking if the `error` key is present in the ES response
		// body.
		_, valueType, _, fetchErr := jsonparser.Get(transformedResponse, queryIDEach, "error")
		if fetchErr == nil && valueType == jsonparser.Object {
			// `error` key is present and we don't need to do anything
			continue
		}

		// Extract the ES Response for the queryID
		valueInBytes, valueType, _, fetchErr := jsonparser.Get(transformedResponse, queryIDEach, "hits", "hits")
		if fetchErr != nil {
			errMsg := fmt.Sprintf("error while parsing response for query with ID '%s' with error: %s", queryIDEach, fetchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			continue
		}

		// Make sure that the value is of type `object`
		if valueType != jsonparser.Array {
			errMsg := fmt.Sprintf("response is not of type array for query with ID '%s', cannot continue", queryIDEach)
			log.Warnln(logTag, ": ", errMsg)
			continue
		}

		valueAsMap := make([]map[string]interface{}, 0)
		unmarshallErr := json.Unmarshal(valueInBytes, &valueAsMap)
		if unmarshallErr != nil {
			errMsg := fmt.Sprintf("error while trying to unmarshall the value into a map for query with ID '%s' with error: %s", queryIDEach, unmarshallErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			continue
		}

		// Throw error if 0 responses are found and `strictSelection` is enabled
		if len(valueAsMap) == 0 && *AIConfigUsed.StrictSelection {
			errMsg := "no hits found in response, cannot build context for AIAnswer"
			log.Warnln(logTag, ": ", errMsg)
			updatedResponse, setErr := jsonparser.Set(transformedResponse, []byte(`"`+errMsg+`"`), queryIDEach, KeyToInject, "error")
			if setErr != nil {
				log.Errorln(logTag, ": ", "error while injecting OpenAI error in response")
			} else {
				transformedResponse = updatedResponse
			}

			// Skip execution
			continue
		}

		// Build the default docTemplate from the first hit only if hits were non 0.
		if AIConfigUsed.DocTemplate == nil && len(valueAsMap) != 0 {
			// Extract the `_source` from the first hit since we have verified that the
			// hits are valid
			hitSourceFirst, valueType, _, sourceFetchErr := jsonparser.Get(valueInBytes, "[0]", "_source")
			if sourceFetchErr != nil {
				errMsg := fmt.Sprint("error while trying to extract the `_source` object from the first hit to build docTemplate: ", sourceFetchErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				continue
			}

			// Make sure source is a map indeed
			if valueType != jsonparser.Object {
				errMsg := fmt.Sprint("invalid `_source` value parsed, not an object!")
				log.Warnln(logTag, ": ", errMsg)
				continue
			}

			// Unmarshal the source into a map to build the default docTemplate
			sourceAsMap := make(map[string]interface{})
			sourceUnmarshalErr := json.Unmarshal(hitSourceFirst, &sourceAsMap)
			if sourceUnmarshalErr != nil {
				errMsg := fmt.Sprint("error while unmarshalling fetched source into a map: ", unmarshallErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				continue
			}

			// Build default docTemplates based on the passed dataFields
			defaultDocTemplate := BuildDefaultDocTemplate(query, sourceAsMap)
			if defaultDocTemplate == "" {
				// Handle this case where the df list might be empty
				errMsg := "cannot build `docTemplate` without `dataField` being passed. Either pass `dataField` or pass `query.AIConfig.docTemplate` value"
				log.Warnln(logTag, ": ", errMsg)
				return transformedResponse, fmt.Errorf(errMsg)
			}

			AIConfigUsed.DocTemplate = &defaultDocTemplate
		}

		// Verify that topDocsForContext is within limit
		validTopDocsValue := int(math.Min(float64(len(valueAsMap)), float64(*AIConfigUsed.TopDocsForContext)))
		AIConfigUsed.TopDocsForContext = &validTopDocsValue

		// Build the ChatGPT request body now
		// Build the messages array to be passed to ChatGPT
		messagesArrToPassChatGPT := make([]map[string]interface{}, 0)
		messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{
			"role":    "system",
			"content": *AIConfigUsed.SystemPrompt,
		})

		documentIdsToChatGPT := make([]string, 0)

		for position, hitEach := range valueAsMap {
			if position == *AIConfigUsed.TopDocsForContext {
				break
			}

			// Extract the `_source` value
			sourceAsMap, asMapOk := hitEach["_source"].(map[string]interface{})
			if !asMapOk {
				log.Warnln(logTag, ": couldn't parse `_source` from hit for position: ", position+1)
				continue
			}

			// Extract the _id since we will show the ID's that were passed to ChatGPT
			idAsStr, asStrOk := hitEach["_id"].(string)
			if !asStrOk {
				log.Warnln(logTag, ": couldn't parse `_id` into a string at position: ", position+1)
				continue
			}

			documentIdsToChatGPT = append(documentIdsToChatGPT, idAsStr)

			messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{
				"role":    "system",
				"content": ParseValuesIntoTemplate(*AIConfigUsed.DocTemplate, sourceAsMap),
			})
		}

		fmt.Println("query Value: ", queryValue)
		queryTemplateToUse := ParseValuesIntoTemplate(*AIConfigUsed.QueryTemplate, map[string]interface{}{"value": queryValue})
		messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{"role": "user", "content": queryTemplateToUse})

		// Make a check to see if the maxTokens value exceeds the limit, if it
		// is specified by the user.
		if AIConfigUsed.MaxTokens != nil || AIConfigUsed.MinTokens != nil {
			trimmedMessages, maxTokensToUse, trimErr := openAIInstance.TrimContextAsPerLimits(messagesArrToPassChatGPT, *AIConfigUsed.MaxTokens, *AIConfigUsed.MinTokens, *AIConfigUsed.StrictSelection)

			if trimErr != nil {
				// Throw an error since this is not allowed.
				errMsg := fmt.Sprint("Error while trimming messages: ", trimErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				updatedResponse, setErr := jsonparser.Set(transformedResponse, []byte(`"`+errMsg+`"`), queryIDEach, KeyToInject, "error")
				if setErr != nil {
					log.Errorln(logTag, ": ", "error while injecting OpenAI error in response")
				} else {
					transformedResponse = updatedResponse
				}

				// Skip execution
				continue
			}

			messagesArrToPassChatGPT = trimmedMessages
			AIConfigUsed.MaxTokens = &maxTokensToUse
		}

		// Now that the request is built, we can send it to ChatGPT

		// We will need a sessionId
		sessionId := openai.GenerateSessionId()

		// Register the response using the passed sessionId
		sessionMap.RegisterResponse(sessionId, userId, indices, documentIdsToChatGPT)

		// Fetch the ChatGPT response in a different function and take care of
		// using the sessionId to populate it accordingly.
		log.Debugln(logTag, ": Executing the ChatGPT part of the request in the background")
		go func(query Query, aiConfig AIConfig) {
			// Use a deferred function to recover from potential panics
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in goroutine: %v\n", r)
					debug.PrintStack()
				}
			}()

			log.Debugln("messages to chatGPT: ", messagesArrToPassChatGPT)
			log.Debugln("ids sent to chatGPT: ", documentIdsToChatGPT)
			log.Debugln("model: ", openAIInstance.GetConfig().GetModel())
			log.Debugln("key: ", openAIInstance.GetConfig().Key())
			log.Debugln("sessionId: ", sessionId)
			log.Debugln("query id for each: ", queryIDEach)
			log.Debugln("max tokens: ", pretty.Formatter(aiConfig))
			log.Debugln("temperature: ", pretty.Formatter(aiConfig))

			err := openai.FetchChatGPTForSessionWithStream(
				sessionId,
				sessionMap,
				openAIInstance.GetConfig().GetModel(),
				messagesArrToPassChatGPT,
				documentIdsToChatGPT,
				openAIInstance.GetConfig().Key(),
				aiConfig.MaxTokens,
				aiConfig.Temperature,
				openAIInstance.GetConfig().GetAPIType(),
				openAIInstance.GetConfig().GetAzureURL(),
				openAIInstance.GetConfig().GetAzureVersion(),
				false,
			)
			// Log the error, if any
			if err != nil {
				log.Printf("Error in FetchChatGPTForSession: %v\n", err)
				debug.PrintStack()
			}
		}(query, AIConfigUsed)

		// Inject the sessionId into the response
		updatedValue, setErr := jsonparser.Set(transformedResponse, []byte(`"`+sessionId+`"`), queryIDEach, SessionIDKeyToInject)
		if setErr != nil {
			errMsg := fmt.Sprint("Something went wrong while trying to inject SessionID for query: ", queryIDEach)
			log.Warnln(logTag, ": ", errMsg)
			continue
		} else {
			transformedResponse = updatedValue
		}

	}

	return transformedResponse, nil
}

// ParseValuesIntoTemplate will parse the passed values into the passed template
// and return a built string.
//
// Values that are not found are going to be substituted with an empty string
func ParseValuesIntoTemplate(template string, values map[string]interface{}) string {
	// Pattern to match the template and find out all the values
	// that need to be dynamically resolved.
	//
	// We support two ways to access the `source` object as of now.
	// - ${source.key}
	// - ${source[key]}
	//
	// Moreover, key's with space are also supported in both the above
	// ways:
	// - ${source.key with space}
	// - ${source[key with space]}
	patternForDynamic := regexp.MustCompile(`\$({|\[)[^,:}]+(}|\])`)
	matches := patternForDynamic.FindAllString(template, -1)

	for _, matchesToReplace := range matches {
		// Value would be in the format ${<str>}
		// Remove the moustaches.
		nakedValue := regexp.MustCompile("\\${|}").ReplaceAllString(matchesToReplace, "")

		// Remove the `source` value from the key if present

		// If the user specified to access using object access.
		//
		// This pattern will check if the user specified pattern is
		// ${source.key}
		// and will accordingly remove the `source.` part from the key.
		isObjectMatch, objectMatchErr := regexp.MatchString("^source\\..+$", nakedValue)
		if objectMatchErr == nil && isObjectMatch {
			nakedValue = strings.Replace(nakedValue, "source.", "", 1)
		}

		// If the user specified item based access.
		//
		// This pattern checks if the user specified the key as
		// ${source[key]} and will accordingly remove the `source[]` part from
		// the string.
		isItemMatch, itemMatchErr := regexp.MatchString("^source\\[.+\\]$", nakedValue)
		if itemMatchErr == nil && isItemMatch {
			nakedValue = strings.Replace(nakedValue, "source[", "", 1)
			nakedValue = strings.Replace(nakedValue, "]", "", -1)
		}

		valueToReplaceWith, valueErr := util.GetNestedValueFromContext(nakedValue, values)
		if valueErr != nil {
			log.Warnln(logTag, ": error while finding value from passed ctx: ", valueErr.Error())
			template = strings.Replace(template, nakedValue, "", -1)
			continue
		}

		// Marshal the value into a string
		valueAsByte, marshalErr := json.Marshal(valueToReplaceWith)
		if marshalErr != nil {
			log.Warnln(logTag, ": error while marshalling value for dynamic input for key: ", valueToReplaceWith)
			continue
		}
		valueAsString := string(valueAsByte)

		// Remove quotes from the value
		unquotedValueAsString, unquoteErr := strconv.Unquote(valueAsString)
		if unquoteErr != nil {
			log.Warnln(logTag, ": error while unquoting marshalled value for dynamic input key: ", valueToReplaceWith, ", with error: ", unquoteErr)
			// No need to throw error since there can be times
			// when type doesn't have quotes at all.
			unquotedValueAsString = valueAsString
		}

		template = strings.Replace(template, matchesToReplace, unquotedValueAsString, -1)
	}

	return template
}

// MakeChatGPTRequest will make the ChatGPT request and return the response accordingly
func MakeChatGPTRequest(model string, messages []map[string]interface{}, apiKey string, maxTokens *int, temperature *float64) (*http.Response, []byte, []byte, error) {
	// Build the request to send to ChatGPT API
	endpointToHit := "/chat/completions"
	URLToHit := OpenAIAPIURL + endpointToHit

	requestBodyAsMap := map[string]interface{}{
		"model":    model,
		"messages": messages,
	}

	// If maxTokens is passed, inject it in the request body
	if maxTokens != nil {
		requestBodyAsMap["max_tokens"] = *maxTokens
	}

	// If temperature is passed, inject it in the request body
	if temperature != nil {
		requestBodyAsMap["temperature"] = *temperature
	}

	bodyAsBytes, bodyMarshalErr := json.Marshal(requestBodyAsMap)
	if bodyMarshalErr != nil {
		return nil, nil, nil, fmt.Errorf("error while marshalling request body into bytes: %s", bodyMarshalErr.Error())
	}

	request, requestCreateErr := http.NewRequest(http.MethodPost, URLToHit, bytes.NewReader(bodyAsBytes))

	if requestCreateErr != nil {
		errMsg := fmt.Sprint("error while creating the request to send OpenAI, ", requestCreateErr)
		log.Errorln(logTag, ": ", errMsg)

		return nil, nil, nil, fmt.Errorf(errMsg)
	}

	// Set the authorization header
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	request.Header.Add("Content-Type", "application/json")

	response, reqErr := util.HTTPClient().Do(request)
	if reqErr != nil {
		errMsg := fmt.Sprint("error while sending request to ChatGPT, ", reqErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, nil, nil, fmt.Errorf(errMsg)
	}

	// Read the body.
	responseInBytes, readErr := ioutil.ReadAll(response.Body)
	if readErr != nil {
		errMsg := fmt.Sprint("error while reading the response body from ChatGPT, ", readErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, nil, nil, fmt.Errorf(errMsg)
	}

	// Verify the status code received, a non 200 OK status code will return an
	// error since it means the embedding failed
	if response.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("non 200 OK status code received from ChatGPT: %s with body: %s", response.Status, string(responseInBytes))
	}

	return response, responseInBytes, bodyAsBytes, nil
}

// BuildDefaultDocTemplate will build the default docTemplate based
// on the passed request body.
//
// The template built will be of type `${dataField} is ${value}, ${dataField} is
// ${value}` etc.
func BuildDefaultDocTemplate(queryBody Query, responseSourceObject map[string]interface{}) string {
	docTemplateFromReq := buildDocTemplateFromRequest(queryBody)

	if docTemplateFromReq != "" {
		return docTemplateFromReq
	}

	// Extract the

	// If the docTemplate was not built properly, we will need
	// to build it using the response body instead.
	return buildDocTemplateFromResponse(responseSourceObject)
}

// buildDocTemplateFromRequest will build the docTemplate by using
// the passed request body
func buildDocTemplateFromRequest(queryBody Query) string {
	// Parse the dataField into something readable
	normalizedFields := NormalizedDataFields(queryBody.DataField, queryBody.FieldWeights)

	if len(normalizedFields) == 0 {
		return ""
	}

	docTemplateFields := make([]string, 0)

	for _, field := range normalizedFields {
		docTemplateFields = append(docTemplateFields, fmt.Sprintf("%s is '${source.%s}'", MakeDataFieldReadable(field.Field), field.Field))
	}

	return strings.Join(docTemplateFields, ", ")
}

// buildDocTemplateFromResponse will build the default docTemplate based
// on the fields from the response
func buildDocTemplateFromResponse(responseObject map[string]interface{}) string {
	// Find out all the keys that are of type string. We can determine the
	// type by typecasting the fields.
	docTemplateFields := make([]string, 0)

	for fieldName, field := range responseObject {
		// Check if the field is of type string
		_, asStrOk := field.(string)
		if !asStrOk {
			continue
		}

		docTemplateFields = append(docTemplateFields, fmt.Sprintf("%s is '${source.%s}'", MakeDataFieldReadable(fieldName), fieldName))
	}

	return strings.Join(docTemplateFields, ", ")
}

// FindValueForAI will try to find the value in the passed
// queryId for the search query.
//
// If `value` is find in the query, it will be given priority
// else it will be fetched from any `react` query that is of type
// search and has a value else a nil value will be returned.
func FindValueForAI(queryToUse Query, rsQuery RSQuery) (string, error) {
	if queryToUse.Value != nil {
		queryValue, queryAsStrOk := (*((queryToUse).Value)).(string)
		if queryAsStrOk {
			return queryValue, nil
		}
	}

	// Since `value` is either not present or not of type string, we
	// will have to check the `react` prop of the query
	if queryToUse.React == nil {
		// We cannot do anything since no `react` value is passed and either
		// value is not passed or is not of type string
		errMsg := ("error while parsing value from query, `value` or `react` not parsable or not present")
		return "", fmt.Errorf(errMsg)
	}

	// If `react` is present, then we can iterate the react values and
	// try to find the first search query that has a valid value field in it and accordingly
	// return it or throw an error.
	value, valueErr, isValuePresent := evalReactPropValue(*queryToUse.React, queryToUse, rsQuery)
	if valueErr != nil {
		return "", valueErr
	}

	if !isValuePresent {
		return "", fmt.Errorf("error while parsing an usable value from the query and `react` of the query")
	}

	return value, nil
}

// evalReactPropValue will evaluate the value from the react properties
// passed in the query
//
// This function returns three values:
// - the value to use
// - if there was some error regarding parsing the value
// - whether or not the value is present
func evalReactPropValue(react interface{}, query Query, rsQuery RSQuery) (string, error, bool) {
	// Check if nested react is present
	reactAsMap, isNestedReact := react.(map[string]interface{})
	if isNestedReact {
		// Handle case of `and`, `or` or `not`
		if reactAsMap["and"] != nil {
			value, valueErr, isValuePresent := evalReactPropValue(reactAsMap["and"], query, rsQuery)
			if isValuePresent && valueErr == nil {
				return value, valueErr, isValuePresent
			}
		}
		if reactAsMap["or"] != nil {
			value, valueErr, isValuePresent := evalReactPropValue(reactAsMap["or"], query, rsQuery)
			if isValuePresent && valueErr == nil {
				return value, valueErr, isValuePresent
			}
		}
		if reactAsMap["not"] != nil {
			value, valueErr, isValuePresent := evalReactPropValue(reactAsMap["not"], query, rsQuery)
			if isValuePresent && valueErr == nil {
				return value, valueErr, isValuePresent
			}
		}
	} else {
		reactAsArr, isArr := react.([]interface{})
		if isArr {
			// Handle case of react array
			for _, reactQuery := range reactAsArr {
				reactAsStr, isStrOk := reactQuery.(string)
				if !isStrOk {
					continue
				}

				value, valueErr, isValuePresent := evalReactPropValue(reactAsStr, query, rsQuery)

				// If `value` is present and there is no error, we use the value
				if isValuePresent && valueErr == nil {
					return value, valueErr, isValuePresent
				}
			}
		} else {
			// Handle case of react being a string
			reactAsStr, asStrOk := react.(string)
			if asStrOk {
				componentQueryInstance := getQueryInstanceByID(reactAsStr, rsQuery)

				// Only consider if the query is of type `search`
				if componentQueryInstance != nil && componentQueryInstance.Type == Search {
					// Query is present
					if componentQueryInstance.Value == nil {
						return "", nil, false
					}

					// It is present, try to parse it into a string
					queryValue, queryAsStrOk := (*((componentQueryInstance).Value)).(string)
					if queryAsStrOk {
						return queryValue, nil, true
					}

					return "", fmt.Errorf("value is not string for query: %s", reactAsStr), true
				}
			}
		}
	}

	return "", fmt.Errorf("value could not parsed into something usable"), true
}
