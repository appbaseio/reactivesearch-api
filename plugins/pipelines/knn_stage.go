package pipelines

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	log "github.com/sirupsen/logrus"
)

// KNNStruct contains the input details
// expected in the knn stage
type KNNStruct struct {
	Backend *querytranslate.Backend `json:"backend" jsonschema:"title=Search Backend" jsonschema_description:"Search backend, defaults to 'elasticsearch'."`
	// TODO: Add title and description
	Search *querytranslate.Query `json:"search"`
}

func GetKNNInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&KNNStruct{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// executeKnn executes the kNN response stage depending on the
// inputs passed by the user.
func executeKnn(stage ESPipelineStage, rsAPIRequest *ReactiveSearchQueryContext, parsedInputs *string, scriptContextInBytes []byte) ([]byte, bool, *Error) {
	// Marshal parsedInputs to the KNNStruct
	knnInputs := new(KNNStruct)
	err := json.Unmarshal([]byte(*parsedInputs), &knnInputs)
	if err != nil {
		errMsg := fmt.Sprint("error while unmarshalling passed inputs: ", err)
		log.Warnln(logTag, ": ", errMsg)
		return nil, false, &Error{
			Err: errors.New(errMsg),
		}
	}

	var scriptContext rules.ScriptContext
	unmarshalErr := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if unmarshalErr != nil {
		log.Errorln(logTag, ": error while unmarshalling script context, ", unmarshalErr)
		return nil, false, &Error{
			Err: unmarshalErr,
		}
	}

	// Set default values in the inputs
	setDefaultValues(knnInputs)

	// Validate the passed inputs
	validateErr := validateKnnInputs(*knnInputs)
	if validateErr != nil {
		log.Warnln(logTag, ": error while validating knn inputs: ", validateErr)
		return nil, false, &Error{
			Err: validateErr,
		}
	}

	// Unmarshal the RSAPI body
	var rsAPIBody querytranslate.RSQuery
	err2 := json.Unmarshal([]byte(scriptContext.Request.Body), &rsAPIBody)
	if err2 != nil {
		errMsg := fmt.Sprint("error while unmarshalling request to RS API body, ", err)
		log.Errorln(logTag, ":", errMsg)
		return nil, false, &Error{
			Err: errors.New(errMsg),
		}
	}

	// Raise error if search or queryvector or vectorfield is not
	// passed.
	if knnInputs.Search == nil || knnInputs.Search.QueryVector == nil || knnInputs.Search.VectorDataField == nil {
		errMsg := "queryVector and vectorDataField are required inputs"
		log.Warnln(errMsg)
		return nil, false, &Error{
			Err:  errors.New(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Set the backend as passed by the user
	if rsAPIBody.Settings == nil {
		rsAPIBody.Settings = new(querytranslate.Settings)
	}

	rsAPIBody.Settings.Backend = knnInputs.Backend

	// Set the other fields but only for search type.
	for queryIndex, query := range rsAPIBody.Query {
		// If type is not search, no need to continue
		if query.Type != querytranslate.Search {
			break
		}

		// Set the fields
		//
		// NOTE: Execution will stop if vectorDataField or queryVector is not
		// passed so we can continue without checking.
		query.VectorDataField = knnInputs.Search.VectorDataField
		query.QueryVector = knnInputs.Search.QueryVector

		// Following fields will be set to default if not passed.
		query.Candidates = knnInputs.Search.Candidates
		query.Script = knnInputs.Search.Script

		rsAPIBody.Query[queryIndex] = query
	}

	// Update rsAPIBody
	rsAPIRequest.Put(&rsAPIBody)

	// Marshal the context again.
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

// validateKnnInputs validates the inputs passed by the
// user.
func validateKnnInputs(inputs KNNStruct) error {
	// Validate the backend
	ALLOWED_OS_SCRIPTS := []string{
		"l1",
		"l2",
		"cosinesimil",
		"hammingbit",
	}

	// NOTE: Backend will be validated automatically
	// since it's an enum

	// If backend is opensearch, validate the script
	// since it can one of few defined ones.
	if *inputs.Backend == querytranslate.OpenSearch && inputs.Search != nil && inputs.Search.Script != nil {
		isValidOSScript := false
		for _, script := range ALLOWED_OS_SCRIPTS {
			if inputs.Search != nil && *inputs.Search.Script == script {
				isValidOSScript = true
				break
			}
		}

		if !isValidOSScript {
			return fmt.Errorf("script should be one of `%s` for opensearch", strings.Join(ALLOWED_OS_SCRIPTS, ", "))
		}
	}

	return nil
}

// setDefaultValues sets the default values in the inputs
// if they are not passed
func setDefaultValues(inputs *KNNStruct) {
	// NOTE: Backend will be set automatically
	// to ES

	// If search is not passed, return
	if inputs.Search == nil {
		return
	}

	if inputs.Search.Candidates == nil {
		defaultMaxSize := 10
		inputs.Search.Candidates = &defaultMaxSize
	}

	if inputs.Search.Script == nil {
		defaultScript := querytranslate.GetDefaultScript(*inputs.Backend)
		inputs.Search.Script = &defaultScript
	}

	return
}
