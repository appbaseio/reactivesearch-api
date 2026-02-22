package pipelines

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

const (
	OpenAIAPIURL string = "https://api.openai.com/v1"
)

type OpenAIEmbeddingsInput struct {
	Text           *string `json:"text,omitempty" jsonschema:"title=Text" jsonschema_description:"Text to get the vector for. Eg: 'test string'"`
	Model          *string `json:"model,omitempty" jsonschema:"title=Model" jsonschema_description:"Model to use for getting the vector embeddings. Options can be found at https://platform.openai.com/docs/models. Defaults to 'text-embedding-ada-002'"`
	ApiKey         *string `json:"apiKey,omitempty" jsonschema:"title=API Key" jsonschema_description:"OpenAI API key to be able to access the API"`
	UseWithRSQuery *bool   `json:"useWithReactiveSearchQuery,omitempty" jsonschema:"title=Use With ReactiveSearch Query" jsonschema_description:"When set as true, the output vector will be populated into the ReactiveSearch query where vectorDataField key is present. Defaults to 'false'"`
}

type OpenAIEmbeddingsIndexInput struct {
	Model     *string   `json:"model,omitempty" jsonschema:"title=Model" jsonschema_description:"Model to use for getting the vector embeddings. Options can be found at https://platform.openai.com/docs/models. Defaults to 'text-embedding-ada-002'"`
	ApiKey    *string   `json:"apiKey,omitempty" jsonschema:"title=API Key" jsonschema_description:"OpenAI API key to be able to access the API"`
	InputKeys *[]string `json:"inputKeys,omitempty" jsonschema:"title=Input Keys" jsonschema_description:"Keys from the request body that should be used for getting the embedding."`
	OutputKey *string   `json:"outputKey,omitempty" jsonschema:"title=Output Key" jsonschema_description:"Key to write the vector data to in the request body"`
}

type OpenAIData struct {
	Embedding *[]float64 `json:"embedding,omitempty"`
}

type OpenAIResponse struct {
	Data *[]OpenAIData `json:"data,omitempty"`
}

// GetOpenAIEmbeddingsInputSchema will return the schema for Open AI Embeddings stage
func GetOpenAIEmbeddingsInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&OpenAIEmbeddingsInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// GetOpenAIEmbeddingsIndexInputSchema will return the schema for Open AI Embeddings Index stage
func GetOpenAIEmbeddingsIndexInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&OpenAIEmbeddingsIndexInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeOpenAIEmbeddingsStage will execute the open AI embeddings stage
// by using the inputs and accordingly generate the vector embedding for the
// text.
func executeOpenAIEmbeddingsStage(
	stage ESPipelineStage,
	parsedInputs *string,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	// Parse the inputs
	stageID := getStageID(stage)
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

	// Parse the inputs to OpenAIEmbeddingsInput
	var inputs OpenAIEmbeddingsInput
	inputParseErr := json.Unmarshal([]byte(*parsedInputs), &inputs)
	if inputParseErr != nil {
		log.Warnln(logTag, ": error while parsing inputs to OpenAIEmbeddings input, ", inputParseErr)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Error while trying to parse inputs as OpenAIEmbeddings ones: %s", inputParseErr),
			Code: http.StatusBadRequest,
		}
	}

	// Validate the inputs
	if inputs.ApiKey == nil {
		// API Key is required and cannot be nil
		errMsg := fmt.Sprint("`apiKey` is a required value for OpenAI stage")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Set default model if none is passed
	if inputs.Model == nil {
		defaultModel := "text-embedding-ada-002"
		inputs.Model = &defaultModel
	}

	// One of text or useWithRSQuery will have to be present else we cannot continue
	// with the execution of this stage
	if (inputs.Text == nil || *inputs.Text == "") && (inputs.UseWithRSQuery == nil || !*inputs.UseWithRSQuery) {
		errMsg := fmt.Sprint("one of `text` or `useWithReactiveSearchQuery` needs to be present")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Set the default value for useWithRSQuery if not passed
	if inputs.UseWithRSQuery == nil {
		defaultUseWithRS := false
		inputs.UseWithRSQuery = &defaultUseWithRS
	}

	// If it is a simple text embedding, we don't need to fetch
	// or modify the request body
	if !*inputs.UseWithRSQuery {
		// Use the embedding function to get the embedding and write
		// it to context using the stage ID.
		embeddingForText, embeddingFetchErr := GetEmbeddingsForText(*inputs.Text, *inputs.Model, *inputs.ApiKey)
		if embeddingFetchErr != nil {
			log.Errorln(logTag, ": error while fetching embeddings for passed text", embeddingFetchErr.Err.Error())
			return scriptContextInBytes, false, embeddingFetchErr
		}

		var output interface{}
		output = map[string]interface{}{
			*stageID: embeddingForText,
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

	requestQuery := rsAPIRequest.Get()

	// Handle case where the requestQuery might be nil.
	//
	// If it is `nil`, we will need to get the request body from context
	// and unmarshal it into the rsAPI structure.
	if requestQuery == nil {
		bodyPassed := scriptContext.Request.Body
		var bodyPassedAsRS querytranslate.RSQuery
		unmarshalErr := json.Unmarshal([]byte(bodyPassed), &bodyPassedAsRS)
		if unmarshalErr != nil {
			return scriptContextInBytes, false, &Error{
				Err:  fmt.Errorf("error while unmarshalling request body into RSQuery structure with err: %s", unmarshalErr.Error()),
				Code: http.StatusBadRequest,
			}
		}

		requestQuery = &bodyPassedAsRS
	}

	// Add check to throw error indicating `useWithRSQuery` doesn't work when
	// async is enabled.
	if async {
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("`async` cannot be enabled if `inputs.useWithReactiveSearchQuery` is set to `true`"),
			Code: http.StatusBadRequest,
		}
	}

	// Find the query that has the vectorDataField present and accordingly get
	// the embedding for it and update the request body by setting the
	// embedding in the queryVector field.
	queryIdToUpdate := ""
	queryIndexToUpdate := 0
	inputString := ""

	for queryIndex, queryEach := range requestQuery.Query {
		if queryEach.VectorDataField != nil && *queryEach.VectorDataField != "" {
			// This is our query
			queryIdToUpdate = *queryEach.ID
			queryIndexToUpdate = queryIndex

			// If `inputs.Text` is not valid, only then we find the value,
			// else we can skip the remaining steps here
			if inputs.Text != nil && *inputs.Text != "" {
				inputString = *inputs.Text
				break
			}

			// Convert the value into a string
			var queryValue []string

			// check if query value of string type
			queryAsString, ok := (*queryEach.Value).(string)
			if ok {
				queryValue = []string{queryAsString}
			} else {
				// check if query value is array
				queryAsArray, ok := (*queryEach.Value).([]interface{})
				if ok {
					for _, v := range queryAsArray {
						valueAsString, ok := v.(string)
						if ok && strings.TrimSpace(valueAsString) != "" {
							queryValue = append(queryValue, valueAsString)
						}
					}
				}
			}

			if len(queryValue) == 0 {
				// Throw an error since we should not embed an empty string here
				return scriptContextInBytes, false, &Error{
					Err:  fmt.Errorf("empty string should not be embedded into vector"),
					Code: http.StatusBadRequest,
				}
			}

			// search query is a join of multiple values with space
			inputString = strings.Join(queryValue, " ")

			break
		}
	}

	// Handle case when `inputString` is empty.
	//
	// This probably means that we didn't find any query that has the `vectorField`
	// present.
	//
	// In such a case, we don't need to do anything and silently exit
	if inputString == "" {
		log.Warnln(logTag, ": no query matched the requirements for openAI embedding to be injected, skipping!")
		return scriptContextInBytes, false, nil
	}

	// We have the queryID that will be updated based on the embedding found for
	// the text value.
	embeddingForText, embeddingFetchErr := GetEmbeddingsForText(inputString, *inputs.Model, *inputs.ApiKey)
	if embeddingFetchErr != nil {
		log.Errorln(logTag, ": error while fetching embeddings for passed text", embeddingFetchErr.Err.Error())
		return scriptContextInBytes, false, embeddingFetchErr
	}

	log.Info(logTag, ": updating rs query with ID: ", queryIdToUpdate)
	requestQuery.Query[queryIndexToUpdate].QueryVector = &embeddingForText

	// Update rsAPIBody
	rsAPIRequest.Put(requestQuery)

	// write updated body to context
	rsBodyInBytes, err := json.Marshal(requestQuery)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	scriptContext.Request.Body = string(rsBodyInBytes)

	contextInBytes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}

// executeOpenAIEmbeddingsIndexStage will execute the index stage for OpenAI embeddings
// using the inputs and accordingly inject the embeddings in the desired key
func executeOpenAIEmbeddingsIndexStage(stage ESPipelineStage,
	parsedInputs *string,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	// Parse the inputs
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

	// Parse the inputs to OpenAIEmbeddingsIndex
	var inputs OpenAIEmbeddingsIndexInput
	inputParseErr := json.Unmarshal([]byte(*parsedInputs), &inputs)
	if inputParseErr != nil {
		log.Warnln(logTag, ": error while parsing inputs to OpenAIEmbeddings input, ", inputParseErr)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf("Error while trying to parse inputs as OpenAIEmbeddings ones: %s", inputParseErr),
			Code: http.StatusBadRequest,
		}
	}

	// Validate the inputs
	if inputs.ApiKey == nil {
		// API Key is required and cannot be nil
		errMsg := fmt.Sprint("`apiKey` is a required value for OpenAI stage")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Set default model if none is passed
	if inputs.Model == nil {
		defaultModel := "text-embedding-ada-002"
		inputs.Model = &defaultModel
	}

	// Make sure that both inputKeys is present since
	// this stage cannot continue without the keys.
	if inputs.InputKeys == nil || len(*inputs.InputKeys) == 0 {
		errMsg := fmt.Sprint("`inputKeys` is a required value for OpenAI Indexing stage")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// If `outputKey` is specified, then this stage cannot run with
	// async enabled.
	if inputs.OutputKey != nil && *inputs.OutputKey != "" && async {
		errMsg := fmt.Sprint("async cannot be `true` when `outputKey` is specified")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Fetch the request body and find the text to get the embeddings for
	bodyPassedAsString := scriptContext.Request.Body

	// Unmarshal the body into a map[string]interface{}
	bodyAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal([]byte(bodyPassedAsString), &bodyAsMap)
	if unmarshalErr != nil {
		errMsg := fmt.Sprint("error while unmarshaling body into an object. Is it an object? Err is: ", unmarshalErr.Error())
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Extract all the keys passed in inputKeys, the behavior is to not throw
	// an error if a key is not found in the object.
	//
	// However, if all the keys turn out to be not present in the request body, we
	// will throw an error since the `text` will turn out to be just an empty string.
	textValues := make([]string, 0)

	for _, textKey := range *inputs.InputKeys {
		textValue := bodyAsMap[textKey]
		if textValue == nil {
			log.Warnln(logTag, ": ", fmt.Sprintf("key `%s` is not present in the request body object", textKey))
			continue
		}

		// Convert the value to a string and if it fails, ignore it.
		textValueAsStr, asStrOk := textValue.(string)
		if !asStrOk {
			log.Warnln(logTag, ": ", fmt.Sprintf("key `%s` is not a string, ignoring", textKey))
			continue
		}

		textValues = append(textValues, textValueAsStr)
	}

	// If all the keys were invalid, we would get an empty array, so we need to throw an error
	// for that.
	if len(textValues) == 0 {
		errMsg := fmt.Sprint("All keys passed in `inputKeys` are either not present or are not string in the request body, cannot continue")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Join the keys using a `,` and make sure that the final string is not an empty string.
	textToEmbed := strings.Join(textValues, ",")
	if strings.TrimSpace(strings.Replace(textToEmbed, ",", "", -1)) == "" {
		errMsg := fmt.Sprint("Final text generated from the passed keys is an empty string, cannot continue")
		log.Warnln(logTag, ": ", errMsg)
		return scriptContextInBytes, false, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Get the embeddings for the text now
	embeddingsForText, embeddingsFetchErr := GetEmbeddingsForText(textToEmbed, *inputs.Model, *inputs.ApiKey)
	if embeddingsFetchErr != nil {
		log.Errorln(logTag, ": error while fetching embeddings for passed text", embeddingsFetchErr.Err.Error())
		return scriptContextInBytes, false, embeddingsFetchErr
	}

	// If the `outputKey` is not specified, we will inject the embeddings into the context.
	if inputs.OutputKey == nil || strings.TrimSpace(*inputs.OutputKey) == "" {
		stageId := getStageID(stage)
		var output interface{}
		output = map[string]interface{}{
			*stageId: embeddingsForText,
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

	// If `outputKey` is specified, we can update the request body directly.
	bodyAsMap[*inputs.OutputKey] = embeddingsForText

	// write updated body to context
	updatedBodyInBytes, err := json.Marshal(bodyAsMap)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	scriptContext.Request.Body = string(updatedBodyInBytes)

	contextInBytes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}

// GetEmbeddingsForText will get the vector embeddings for the
// passed text value.
func GetEmbeddingsForText(text string, model string, apiKey string) ([]float64, *Error) {
	URLToHit := OpenAIAPIURL + "/embeddings"
	bodyToSend := map[string]interface{}{
		"model": model,
		"input": text,
	}

	// Marshal the body
	bodyAsBytes, bodyMarshalErr := json.Marshal(bodyToSend)
	if bodyMarshalErr != nil {
		return nil, &Error{
			Err:  fmt.Errorf("error while marshalling body to send to OpenAI: %s", bodyMarshalErr.Error()),
			Code: http.StatusInternalServerError,
		}
	}

	request, requestCreateErr := http.NewRequest(http.MethodPost, URLToHit, bytes.NewReader(bodyAsBytes))

	if requestCreateErr != nil {
		errMsg := fmt.Sprint("error while creating the request to send OpenAI, ", requestCreateErr)
		log.Errorln(logTag, ": ", errMsg)

		return nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// Set the authorization header
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	request.Header.Add("Content-Type", "application/json")

	response, reqErr := util.HTTPClient().Do(request)
	if reqErr != nil {
		errMsg := fmt.Sprint("error while sending request to OpenAI, ", reqErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// Read the body.
	responseInBytes, readErr := ioutil.ReadAll(response.Body)
	if readErr != nil {
		errMsg := fmt.Sprint("error while reading the response body from OpenAI, ", readErr)
		log.Warnln(logTag, ": ", errMsg)

		return nil, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusInternalServerError,
		}
	}

	// Verify the status code received, a non 200 OK status code will return an
	// error since it means the embedding failed
	if response.StatusCode != http.StatusOK {
		return nil, &Error{
			Err:  fmt.Errorf("non 200 OK status code received from OpenAI: %s with body: %s", response.Status, string(responseInBytes)),
			Code: http.StatusInternalServerError,
		}
	}

	// Parse the embeddings from the response into the structure
	var openAIResponse OpenAIResponse
	unmarshalErr := json.Unmarshal(responseInBytes, &openAIResponse)

	if unmarshalErr != nil {
		return nil, &Error{
			Err:  fmt.Errorf("error while unmarshalling received body from OpenAI: %s", unmarshalErr.Error()),
			Code: http.StatusInternalServerError,
		}
	}

	// Check if the Data array is not empty
	if openAIResponse.Data == nil || len(*openAIResponse.Data) == 0 {
		return nil, &Error{
			Err:  fmt.Errorf("response doesn't contain `data` key or has an empty array in `data` key, cannot extract embeddings"),
			Code: http.StatusInternalServerError,
		}
	}

	dataObjectToUse := (*openAIResponse.Data)[0]
	if dataObjectToUse.Embedding == nil || len(*dataObjectToUse.Embedding) == 0 {
		return nil, &Error{
			Err:  fmt.Errorf("response deosn't contain `data.embedding` key or has an empty array in that key, cannot extract embedding"),
			Code: http.StatusInternalServerError,
		}
	}

	return *dataObjectToUse.Embedding, nil
}
