package pipelines

import (
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	log "github.com/sirupsen/logrus"
)

// ReqModificationHandler is a type of function that
// modifies the request
type ReqModificationHandler func(req *querytranslate.RSQuery, data *string) error

// ResModificationHandler is a type of function that modifies the response
type ResModificationHandler func(responseBody []byte, data *string) ([]byte, error)

// Promote results input schema
type PromotedResultInput struct {
	Data []rules.PromotedResult `json:"data,omitempty" jsonschema:"title=Data,required" jsonschema_description:"An array of objects to promote, for e.g, '[{ 'doc': { '_id': 'id_1', '_source': { 'title': 'id_1' } }, 'position': 10 }]'."`
}

func GetPromotedResultsInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&PromotedResultInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executePromoteResult executes the promote result stage
// which is a prebuilt query rules stage.
//
// This is an exception to the above generic types defined so
// it is defined explicitly.
func executePromoteResult(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte, rsAPIRequest *ReactiveSearchQueryContext) ([]byte, bool, *Error) {
	var scriptContext rules.ScriptContext
	err := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	var rsAPIBody querytranslate.RSQuery

	rsQuery := rsAPIRequest.Get()
	if rsQuery != nil {
		rsAPIBody = *rsQuery
	} else {
		err2 := json.Unmarshal([]byte(scriptContext.Request.Body), &rsAPIBody)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			return nil, false, &Error{
				Err: err2,
			}
		}
	}

	if parsedInputs != nil {
		// Convert the inputs to map
		var inputsMapped = new(map[string]interface{})
		marshalErr := json.Unmarshal([]byte(*parsedInputs), inputsMapped)
		if marshalErr != nil {
			log.Errorln(logTag, ": error while parsing inputs to map: ", marshalErr)
			return scriptContextInBytes, false, &Error{
				Err: marshalErr,
			}
		}
		if data, ok := (*inputsMapped)["data"]; ok {
			dataAsBytes, err := json.Marshal(data)
			if err != nil {
				log.Warnln(logTag, "skipping since inputs.data cannot be marshalled to a string, ", err)
				return scriptContextInBytes, false, &Error{
					Err: err,
				}
			}
			dataAsString := string(dataAsBytes)

			responseWithRule, err := rules.ApplyPromotedResult(rsAPIBody, []byte(scriptContext.Response.Body), &dataAsString)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, false, &Error{
					Err: err,
				}
			}
			scriptContext.Response.Body = string(responseWithRule)
			contextInBytes, err := json.Marshal(scriptContext)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, false, &Error{
					Err: err,
				}
			}
			return contextInBytes, false, nil
		}
	}

	return scriptContextInBytes, false, nil
}

// Hide results input schema
type HideResultInput struct {
	Data []string `json:"data,omitempty" jsonschema:"title=Data,required" jsonschema_description:"An array of document ids to hide from results. For e.g, '['id_1', 'id_2']'"`
}

func GetHideResultsInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&HideResultInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeHideResult will execute the hide result stage
// which is a query rules stage prebuilt
func executeHideResult(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte) ([]byte, bool, *Error) {
	return executeStageWithResponseModification(stage, parsedInputs, scriptContextInBytes, rules.ApplyHideResult)
}

// custom data input schema
type CustomDataInput struct {
	Data interface{} `json:"data,omitempty" jsonschema:"title=Data,required" jsonschema_description:"Custom data to be returned in response, for e.g, { facets: { color: 'red', brand: 'Apple' }}."`
}

func GetCustomDataInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&CustomDataInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeCustomData will execute the apply custom data
// stage which is a prebuilt rules stage.
func executeCustomData(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte) ([]byte, bool, *Error) {
	return executeStageWithResponseModification(stage, parsedInputs, scriptContextInBytes, rules.ApplyCustomData)
}

// executeStageWithResponseModification will execute the stage with response
// modification.
func executeStageWithResponseModification(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte, handlerFunc ResModificationHandler) ([]byte, bool, *Error) {
	if parsedInputs == nil {
		log.Warnln(logTag, "skipping since inputs is not passed")
		return scriptContextInBytes, false, nil
	}

	var scriptContext rules.ScriptContext
	err := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}

	// Convert the inputs to map
	var inputsMapped = new(map[string]interface{})
	marshalErr := json.Unmarshal([]byte(*parsedInputs), inputsMapped)
	if marshalErr != nil {
		log.Errorln(logTag, ": error while parsing inputs to map: ", marshalErr)
		return scriptContextInBytes, false, &Error{
			Err: marshalErr,
		}
	}

	data, ok := (*inputsMapped)["data"]
	if !ok {
		log.Warnln(logTag, "skipping since inputs.data is invalid")
		return scriptContextInBytes, false, nil
	}

	// Convert the data to a string
	dataAsBytes, err := json.Marshal(data)
	if err != nil {
		log.Warnln(logTag, "skipping since inputs.data cannot be marshalled to a string, ", err)
		return scriptContextInBytes, false, &Error{
			Err: err,
		}
	}
	dataAsString := string(dataAsBytes)

	responseWithRule, err := handlerFunc([]byte(scriptContext.Response.Body), &dataAsString)
	if err != nil {
		log.Errorln(logTag, ": error while running custom data: ", err)
		return scriptContextInBytes, false, &Error{
			Err: err,
		}
	}

	scriptContext.Response.Body = string(responseWithRule)
	contextInBytes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ": error while marshalling custom data body: ", err)
		return nil, false, &Error{
			Err: err,
		}
	}

	return contextInBytes, false, nil
}

// replace search input schema
type ReplaceSearchInput struct {
	Data string `json:"data,omitempty" jsonschema:"title=Data,required" jsonschema_description:"Search query to replace the request's query, for e.g, 'iphoneX'."`
}

func GetReplaceSearchInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&ReplaceSearchInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeReplaceSearch will apply the replace search term
// query rules stage by running it as a pipeline stage
func executeReplaceSearch(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte) ([]byte, bool, *Error) {
	return executeStageWithRequestModification(stage, parsedInputs, scriptContextInBytes, rules.ApplyReplaceSearch)
}

// Add filter input schema
type AddFilterInput struct {
	Data map[string]interface{} `json:"data,omitempty" jsonschema:"title=Data,required" jsonschema_description:"To add a custom filter to the query, for e.g, '{ 'authors.keyword': 'Simone Elkeles' }' would filters books by author."`
}

func GetAddFilterInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&AddFilterInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// exectueAddFilter will apply the filter using the
// apply filter which is a prebuilt query rule
func executeAddFilter(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte) ([]byte, bool, *Error) {
	return executeStageWithRequestModification(stage, parsedInputs, scriptContextInBytes, rules.ApplyFilters)
}

// Remove words input schema
type RemoveWordsInput struct {
	Data []string `json:"data,omitempty" jsonschema:"title=Data,required" jsonschema_description:"To remove a list of words from the search query, for e.g, ['iphone']."`
}

func GetRemoveWordsInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&RemoveWordsInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeRemoveWords will execute the remove words rule
// which is prebuilt into query rules
func executeRemoveWords(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte) ([]byte, bool, *Error) {
	return executeStageWithRequestModification(stage, parsedInputs, scriptContextInBytes, rules.ApplyRemoveWords)
}

// Replace words input schema
type ReplaceWordsInput struct {
	Data map[string]string `json:"data,omitempty" jsonschema:"title=Data,required" jsonschema_description:"To replace words from search query, for e.g, { 'iphone': 'iphoneX' }."`
}

func GetReplaceWordsInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&ReplaceWordsInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeReplaceWords will execute the replace words rule
// which is prebuilt inot query rules
func executeReplaceWords(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte) ([]byte, bool, *Error) {
	return executeStageWithRequestModification(stage, parsedInputs, scriptContextInBytes, rules.ApplyReplaceWords)
}

// executeStageWithRequestModification will execute the passed stage
// that modifies the request.
func executeStageWithRequestModification(stage ESPipelineStage, parsedInputs *string, scriptContextInBytes []byte, handlerFunc ReqModificationHandler) ([]byte, bool, *Error) {
	if parsedInputs == nil {
		log.Warnln(logTag, ": skipping this stage since inputs is not passed")
		return scriptContextInBytes, false, nil
	}

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

	// Convert the inputs to map
	var inputsMapped = new(map[string]interface{})
	marshalErr := json.Unmarshal([]byte(*parsedInputs), inputsMapped)
	if marshalErr != nil {
		log.Errorln(logTag, ": error while parsing inputs to map: ", marshalErr)
		return scriptContextInBytes, false, &Error{
			Err: marshalErr,
		}
	}

	data, ok := (*inputsMapped)["data"]
	if !ok {
		log.Warnln(logTag, " skipping sicne inputs.data is invalid")
		return scriptContextInBytes, false, nil
	}

	// Convert the data to a string
	dataAsBytes, err := json.Marshal(data)
	if err != nil {
		log.Warnln(logTag, " skipping since inputs.data cannot be marshalled to a string, ", err)
		return scriptContextInBytes, false, &Error{
			Err: err,
		}
	}
	dataAsString := string(dataAsBytes)

	handleErr := handlerFunc(&rsAPIBody, &dataAsString)
	if handleErr != nil {
		log.Errorln(logTag, ": error while calling handler: ", handleErr)
		return scriptContextInBytes, false, &Error{
			Err: handleErr,
		}
	}

	// Marshal the request body
	requestWithRule, marshalErr := json.Marshal(rsAPIBody)
	if marshalErr != nil {
		log.Errorln(logTag, "error while marshalling rs body after calling handler: ", marshalErr)
		return scriptContextInBytes, false, &Error{
			Err: marshalErr,
		}
	}

	scriptContext.Request.Body = string(requestWithRule)
	contextInBytes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ": error while marshalling script context: ", err)
		return scriptContextInBytes, false, &Error{
			Err: err,
		}
	}

	return contextInBytes, false, nil
}
