package pipelines

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/openai"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	log "github.com/sirupsen/logrus"
)

type AIAnswerInput struct {
	SystemPrompt      *string  `json:"systemPrompt,omitempty" jsonschema:"title=System Prompt" jsonschema_description:"(optional) First message that will be sent to ChatGPT from the system. Defaults to: You are a helpful assistant"`
	TopDocsForContext *int     `json:"topDocsForContext,omitempty" jsonschema:"title=Top Docs For Context" jsonschema_description:"Number of documents from the top hits to pass to ChatGPT as context for the query. This value is capped to the minimum value of size (hits fetched in a request) and 100. Defaults to 3"`
	DocTemplate       *string  `json:"docTemplate,omitempty" jsonschema:"title=Doc Template" jsonschema_description:"(optional) Template for building the context message for each hit. Supports special character 'source' that refers to the document '_source' field. Eg: '${source.text} is ${source.summary} with url as ${source.url}'"`
	QueryTemplate     *string  `json:"queryTemplate,omitempty" jsonschema:"title=Query Template" jsonschema_description:"(optional) Template for the query that is passed to ChatGPT as the question. Supports special character 'value' that refers to the 'value' field passed in the query object, e.g. Answer the query: '${value}'. Think step-by-step, cite the source after the answer and ensure the source is from the provided context."`
	MaxTokens         *int     `json:"maxTokens,omitempty" jsonschema:"title=Maximum Tokens" jsonschema_description:"(optional) Maximum number of tokens to pass to ChatGPT. Read more about it here: https://platform.openai.com/docs/api-reference/chat/create#chat/create-max_tokens"`
	Temperature       *float64 `json:"temperature,omitempty" jsonschema:"title=Temperature" jsonschema_description:"(optional) Temperature to pass to ChatGPT. Defaults to 1. Read more about it here: https://platform.openai.com/docs/api-reference/chat/create#chat/create-temperature"`
	Model             *string  `json:"model,omitempty" jsonschema:"title=Model" jsonschema_description:"Model to use for getting the vector embeddings. Options can be found at https://platform.openai.com/docs/models. Defaults to 'gpt-3.5-turbo'"`
	ApiKey            *string  `json:"apiKey,omitempty" jsonschema:"title=API Key" jsonschema_description:"(mandatory) OpenAI API key to access the API"`
	QueryID           *string  `json:"queryId,omitempty" jsonschema:"title=Query ID" jsonschema_description:"(optional) ID of the query where hits are going to be extracted from. This should be rarely needed, as it is picked from the ReactiveSearch query body to a 'search' type of query, with a fallback to a 'suggestion' type of query."`
}

// GetAIAnswerInputSchema will return the schema for the AIAnswer stage
func GetAIAnswerInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&AIAnswerInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// RSAPIResponseToExtractHit
type RSAPIResponseToExtractHit struct {
	Hits *RSAPIResponseNestedHit `json:"hits,omitempty"`
}

type RSAPIResponseNestedHit struct {
	Hits *[]RSAPIHitSource `json:"hits,omitempty"`
}

type RSAPIHitSource struct {
	ID     *string                 `json:"_id,omitempty"`
	Source *map[string]interface{} `json:"_source,omitempty"`
}

// executeAIAnswerStage will execute the AI Answer stage and accordingly inject
// a response into the context.
func executeAIAnswerStage(
	stage ESPipelineStage,
	parsedInputs *string,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	// Parse the inputs
	// stageID := getStageID(stage)
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
	// We cannot continue if inputs are not passed
	if parsedInputs == nil {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Inputs are required for the stage, cannot continue without them"),
			Code: http.StatusBadRequest,
		}
	}

	// Parse the inputs to AIAnswerInput
	var inputs AIAnswerInput
	inputParseErr := json.Unmarshal([]byte(*parsedInputs), &inputs)
	if inputParseErr != nil {
		log.Warnln(logTag, ": error while parsing inputs to AIAnswer input, ", inputParseErr)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Error while trying to parse inputs as AIAnswer ones: %s", inputParseErr),
			Code: http.StatusBadRequest,
		}
	}

	// Validate the inputs
	if inputs.ApiKey == nil {
		// API Key is required and cannot be nil
		errMsg := fmt.Sprint("`apiKey` is a required value for AIAnswer stage")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Don't throw error if docTemplate is not passed
	// We will build the default template from the response

	// Set default model if none is passed
	if inputs.Model == nil {
		defaultModel := "gpt-3.5-turbo"
		inputs.Model = &defaultModel
	}

	// Default to systemPrompt if not passed
	if inputs.SystemPrompt == nil {
		defaultSystemPrompt := "You're a helpful assistant."
		inputs.SystemPrompt = &defaultSystemPrompt
	}

	// Defaults to maxTokens if not passed
	if inputs.MaxTokens == nil {
		defaultMaxTokens := 300
		inputs.MaxTokens = &defaultMaxTokens
	}

	// Set default values for queryTemplate
	if inputs.QueryTemplate == nil {
		defaultQueryTemplate := "Answer the query: '${value}'. Think step-by-step, cite the source after the answer and ensure the source is from the provided context."
		inputs.QueryTemplate = &defaultQueryTemplate
	}

	// Extract the RS API Query.
	//
	// Since this stage is always expected to run after RS API Stage, this
	// value should be populated by this time.
	requestQuery := rsAPIRequest.Get()

	// If the value is not populated yet, we will have to throw an error
	// indicating that this stage should be run after the reactivesearchQuery stage.
	if requestQuery == nil {
		errMsg := "error while extracting ReactiveSearch Query from context. This stage should always be after the `reactivesearchQuery` has been executed."
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Try to extract the queryID automatically if it's not present else throw an error indicating
	// that it could not be extracted.
	if inputs.QueryID == nil {
		queryIdFound := ""
		fallbackQueryId := ""

		for _, queryEach := range requestQuery.Query {
			// Try to find the first query that matches the criteria.
			//
			// Preferred option is to pickup the query that has `enableAI`
			// as `true`, else fallback to first search query.
			if queryEach.EnableAI != nil && *queryEach.EnableAI {
				queryIdFound = *queryEach.ID
				break
			}

			if fallbackQueryId == "" {
				if queryEach.Type == querytranslate.Search {
					fallbackQueryId = *queryEach.ID
				}
			}
		}

		if queryIdFound == "" && fallbackQueryId != "" {
			queryIdFound = fallbackQueryId
		}

		if queryIdFound == "" {
			// We were not able to automatically extract a query that matched the
			// criteria
			errMsg := fmt.Errorf("`queryId` could not be extracted automatically since no query with type `search` or `suggestion` was found, please specify it through inputs!")
			log.Warnln(logTag, ": ", errMsg)

			// No need to throw an error here, silently skip
			return scriptContextInBytes, false, nil
		}

		inputs.QueryID = &queryIdFound
	}

	// Extract the response and accordingly verify the response is valid
	//
	// We want to make sure that it's not null and elasticsearchQuery was run
	// before that.
	responseBody := scriptContext.Response.Body

	if responseBody == "" {
		errMsg := fmt.Errorf("error while parsing response body, it is empty. This stage should always be after the `elasticsearchQuery` stage has been executed.")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusBadRequest,
		}
	}

	responseBodyAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal([]byte(responseBody), &responseBodyAsMap)
	if unmarshalErr != nil {
		errMsg := fmt.Errorf("error while unmarshalling response body into object: %s", unmarshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusInternalServerError,
		}
	}

	// Extract the hits based on the query ID passed
	bodyQueryId, queryIdPresent := responseBodyAsMap[*inputs.QueryID]
	if !queryIdPresent {
		errMsg := fmt.Sprintf("`%s` is not present inside response body, cannot continue!", *inputs.QueryID)
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	bodyQueryIdAsMap, asMapOk := bodyQueryId.(map[string]interface{})
	if !asMapOk {
		errMsg := fmt.Sprintf("`%s` is not an object!", *inputs.QueryID)
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Marshal the map and unmarshal into our structure to make it easier
	// to iterate it
	queryIdBodyAsBytes, marshalErr := json.Marshal(bodyQueryIdAsMap)
	if marshalErr != nil {
		errMsg := fmt.Errorf("error while marshalling query ID object into bytes: %s", marshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusInternalServerError,
		}
	}

	var rsAPIStructure RSAPIResponseToExtractHit
	structureUnmarshalErr := json.Unmarshal(queryIdBodyAsBytes, &rsAPIStructure)
	if structureUnmarshalErr != nil {
		errMsg := fmt.Sprintf("error while unmarshalling query ID object into custom structure: %s", structureUnmarshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	if rsAPIStructure.Hits == nil || rsAPIStructure.Hits.Hits == nil {
		errMsg := fmt.Errorf("ReactiveSearch Response is not in proper format!")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusInternalServerError,
		}
	}

	totalHitsPresent := len(*rsAPIStructure.Hits.Hits)

	if inputs.TopDocsForContext == nil {
		defaultTopDocCount := 3
		inputs.TopDocsForContext = &defaultTopDocCount
	} else {
		// Get the topDocsForContext in a valid value
		validTopDocsValue := int(math.Min(float64(totalHitsPresent), float64(*inputs.TopDocsForContext)))
		inputs.TopDocsForContext = &validTopDocsValue
	}

	// Add the query/question as well
	var queryToUse *querytranslate.Query = nil
	for _, query := range requestQuery.Query {
		if *query.ID == *inputs.QueryID {
			queryToUse = &query
		}
	}

	if queryToUse == nil {
		errMsg := fmt.Errorf("error while trying to extract the query with the passed queryID: %s", *inputs.QueryID)
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusBadRequest,
		}
	}

	// Build the default docTemplate if not present
	if inputs.DocTemplate == nil {
		// Make sure that the hits are not empty
		if rsAPIStructure.Hits.Hits == nil || len(*rsAPIStructure.Hits.Hits) == 0 {
			// Handle this scenario
			errMsg := fmt.Errorf("response are not parsable, cannot build default docTemplate value!")
			log.Warnln(logTag, ": ", errMsg)
			return scriptContextInBytes, false, &Error{
				Err:  errMsg,
				Code: http.StatusBadRequest,
			}
		}

		// Build default docTemplates based on the passed dataFields
		defaultDocTemplate := querytranslate.BuildDefaultDocTemplate(*queryToUse, *(*rsAPIStructure.Hits.Hits)[0].Source)
		if defaultDocTemplate == "" {
			// Handle this case where the df list might be empty
			errMsg := "cannot build `docTemplate` without `dataField` being passed. Either pass `dataField` or pass `AIConfig.docTemplate` value"
			log.Warnln(logTag, ": ", errMsg)
			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusBadRequest,
			}
		}

		inputs.DocTemplate = &defaultDocTemplate
	}

	// Build the messages array to be passed to ChatGPT
	messagesArrToPassChatGPT := make([]map[string]interface{}, 0)
	messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{
		"role":    "system",
		"content": *inputs.SystemPrompt,
	})

	// Iterate the hits and build the context
	for position, hit := range *rsAPIStructure.Hits.Hits {
		if position == *inputs.TopDocsForContext {
			break
		}

		messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{
			"role":    "system",
			"content": querytranslate.ParseValuesIntoTemplate(*inputs.DocTemplate, *hit.Source),
		})
	}

	// Add check to skip the stage if query is not present.
	if queryToUse.Value == nil {
		// We don't need to throw an error, instead skip the stage without doing anything
		errMsg := fmt.Sprint("cannot continue execution if `value` is not present for query")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, nil
	}

	queryValue, queryAsStrOk := (*((*queryToUse).Value)).(string)
	if !queryAsStrOk {
		errMsg := fmt.Errorf("error while parsing value into a string to use in the template")
		log.Warnln(logTag, ": ", errMsg)
		queryValue = ""
	}

	queryTemplateToUse := querytranslate.ParseValuesIntoTemplate(*inputs.QueryTemplate, map[string]interface{}{"value": queryValue})
	messagesArrToPassChatGPT = append(messagesArrToPassChatGPT, map[string]interface{}{"role": "user", "content": queryTemplateToUse})

	// Make the ChatGPT request
	_, responseInBytes, chatGPTReqBodyInBytes, _, _, chatGPTErr := openai.MakeChatGPTRequest(*inputs.Model, messagesArrToPassChatGPT, *inputs.ApiKey, inputs.MaxTokens, inputs.Temperature)
	if chatGPTErr != nil {
		errMsg := fmt.Errorf("error while sending request to chatGPT: %s", chatGPTErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusInternalServerError,
		}
	}

	// Unmarshall the bytes into a map so that it can be injected.
	chatGPTResponseAsMap := make(map[string]interface{})
	chatGPTResponseUnmarshallErr := json.Unmarshal(responseInBytes, &chatGPTResponseAsMap)
	if chatGPTResponseUnmarshallErr != nil {
		errMsg := fmt.Errorf("error while unmarshalling chatGPT response into map: %s", chatGPTResponseUnmarshallErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusInternalServerError,
		}
	}

	bodyQueryIdAsMap[querytranslate.KeyToInject] = chatGPTResponseAsMap
	responseBodyAsMap[*inputs.QueryID] = bodyQueryIdAsMap

	updatedBodyInBytes, marshalErr := json.Marshal(responseBodyAsMap)
	if marshalErr != nil {
		errMsg := fmt.Errorf("error while marshalling updated body into bytes: %s", marshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  errMsg,
			Code: http.StatusInternalServerError,
		}
	}

	scriptContext.Response.Body = string(updatedBodyInBytes)
	scriptContext.Request.Body = string(chatGPTReqBodyInBytes)

	contextInBytes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}
