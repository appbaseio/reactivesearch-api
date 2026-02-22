package pipelines

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	log "github.com/sirupsen/logrus"
)

type SearchboxPreferencesStruct struct {
	Id *string `json:"id" jsonschema:"title=SearchBox Id" jsonschema_description:"Searchbox Id to apply the searchbox preferences for suggestion type of requests."`
}

func GetSearchboxPreferencesInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&SearchboxPreferencesStruct{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

func executeSearchPreferencesStage(
	stage ESPipelineStage,
	parsedInputs *string,
	scriptContextInBytes []byte,
	rsAPIRequest *ReactiveSearchQueryContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	if parsedInputs == nil {
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
	var rsAPIBody querytranslate.RSQuery
	err2 := json.Unmarshal([]byte(scriptContext.Request.Body), &rsAPIBody)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}
	// parse stage inputs
	// Unmarshal the input into a map[string]interface{} since it's
	// a string now.
	var inputsMapped = make(map[string]interface{})
	marshalErr := json.Unmarshal([]byte(*parsedInputs), &inputsMapped)
	if marshalErr != nil {
		log.Errorln(logTag, ": error while unmarshalling inputs: ", marshalErr)
		return nil, false, &Error{
			Err: marshalErr,
		}
	}

	inputAsBytes, err := json.Marshal(inputsMapped)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	var searchboxPreferencesSettings SearchboxPreferencesStruct
	err3 := json.Unmarshal(inputAsBytes, &searchboxPreferencesSettings)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return nil, false, &Error{
			Err: err3,
		}
	}
	// apply searchbox settings
	for i, query := range rsAPIBody.Query {
		switch query.Type {
		case querytranslate.Suggestion:
			if searchboxPreferencesSettings.Id != nil {
				if rsAPIBody.Query[i].SearchBoxId == nil || strings.TrimSpace(*rsAPIBody.Query[i].SearchBoxId) == "" {
					rsAPIBody.Query[i].SearchBoxId = searchboxPreferencesSettings.Id
				}
			}
		}
	}
	// Update rsAPIBody
	rsAPIRequest.Put(&rsAPIBody)

	// write updated body to context
	rsBodyInBytes, err := json.Marshal(rsAPIBody)
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
