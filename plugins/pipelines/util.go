package pipelines

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antonmedv/expr"
	"github.com/appbaseio-confidential/reactivesearch/model/acl"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	"github.com/appbaseio-confidential/reactivesearch/util"
	units "github.com/bcicen/go-units"
	"github.com/invopop/jsonschema"
	"github.com/kr/pretty"
	"github.com/lithammer/shortuuid/v4"
	"github.com/robfig/cron"
	"github.com/sergi/go-diff/diffmatchpatch"
	log "github.com/sirupsen/logrus"
)

// Declare a ESPipelineDoc
type ESPipelineDocIn struct {
	ID              *string                `json:"id,omitempty" jsonschema:"title=Pipeline ID" jsonschema_description:"Auto-generated unique identifier for pipeline."`
	Enabled         *bool                  `json:"enabled,omitempty" jsonschema:"title=Enable Pipeline" jsonschema_description:"Set as 'false' to disable a Pipeline. Defaults to 'true'.\n\nThis field can be used to disable a pipeline to test some effect or in scenarios where the pipeline might need to be disabled."`
	Description     *string                `json:"description,omitempty" jsonschema:"title=Description" jsonschema_description:"Description of pipeline.\n\nThis can be a brief explanation of what the pipeline does, this is useful for better understanding of what the pipeline is actually doing."`
	Priority        *int64                 `json:"priority,omitempty" jsonschema:"title=Priority" jsonschema_description:"In case of a conflict in pipeline routes, the pipeline with highest priority would get invoked.\n\nPriority is an important and useful field. This can be used to set the order in which pipelines with same route can be executed when a request comes."`
	Routes          *[]ESPipelineRoutes    `json:"routes,omitempty" jsonschema:"title=Routes,required" jsonschema_description:""`
	Envs            map[string]interface{} `json:"envs,omitempty" jsonschema:"title=Pipeline Environments" jsonschema_description:"Useful to define custom environment variables which could be accessed by stages during pipeline execution.\n\n[Read more about pipeline environments over here](/docs/pipelines/concepts/envs-for-stage)"`
	Trigger         *rules.Trigger         `json:"trigger,omitempty" jsonschema:"title=Trigger Expression" jsonschema_description:""`
	updatedAt       *int64
	createdAt       *int64
	Stages          *[]ESPipelineStageIn `json:"stages,omitempty" jsonschema:"title=Stages,required" jsonschema_description:""`
	GlobalVars      *[]PipelineVar       `json:"global_envs,omitempty" jsonschema:"title=Global Envs" jsonschema_description:"Global Envs will be saved to the cluster and can be used in the pipeline."`
	ValidateContext *string              `json:"validateContext,omitempty"`
}

// Declare the type that will be stored in ES
type ESPipelineDoc struct {
	ID              *string                `json:"id,omitempty"`
	OriginalConfig  *Config                `json:"originalConfig,omitempty"`
	Versions        *[]Version             `json:"version,omitempty"`
	Enabled         *bool                  `json:"enabled,omitempty"`
	Description     *string                `json:"description,omitempty"`
	Priority        *int64                 `json:"priority,omitempty"`
	Routes          *[]ESPipelineRoutes    `json:"routes,omitempty"`
	Envs            map[string]interface{} `json:"envs,omitempty"`
	Trigger         *rules.Trigger         `json:"trigger,omitempty"`
	UpdatedAt       *int64                 `json:"updatedAt,omitempty"`
	CreatedAt       *int64                 `json:"createdAt,omitempty"`
	Stages          *[]ESPipelineStage     `json:"stages,omitempty"`
	LiveVersion     *int                   `json:"live_version,omitempty"`
	ValidateContext *string                `json:"validateContext,omitempty"`
}

// Store details about the original config passed
type Config struct {
	Content         *string `json:"content,omitempty"`
	Type            *string `json:"extension,omitempty"`
	ValidateContext *string `json:"validateContext,omitempty"`
}

// Version will be similar to Config and will contain content
// and type along with version info.
//
// `content` will be a stringified JSON of ESPipelineDoc that should
// be directly unmarshallable into the ESPipelineDoc object.
type Version struct {
	Content     *string `json:"content,omitempty"`
	Version     *int    `json:"version,omitempty"`
	Description *string `json:"description,omitempty"`
}

// Define structure to return config to return
type ConfigOut struct {
	ID                 *string            `json:"id,omitempty"`
	Version            *int               `json:"_version,omitempty"`
	VersionDescription *string            `json:"_version_description,omitempty"`
	IsLive             *bool              `json:"is_live,omitempty"`
	Enabled            *bool              `json:"enabled,omitempty"`
	Priority           *int64             `json:"priority,omitempty"`
	Description        *string            `json:"description,omitempty"`
	Routes             []ESPipelineRoutes `json:"routes,omitempty"`
	UpdatedAt          *int64             `json:"updated_at,omitempty"`
	CreatedAt          *int64             `json:"created_at,omitempty"`
	Content            *string            `json:"content,omitempty"`
	Key                *string            `json:"key,omitempty"`
	Type               *string            `json:"extension,omitempty"`
	Usage              interface{}        `json:"usage,omitempty"`
	ValidateContext    *string            `json:"validateContext,omitempty"`
}

// Handle Routes inside the ESPipelineDoc
type ESPipelineRoutes struct {
	Path       *string        `json:"path,omitempty" jsonschema:"required,title=Path" jsonschema_description:"Route path. For example, '/books-search'.\n\n[More can be read about the route matching process here](/docs/pipelines/concepts/execution-process)."`
	Method     *string        `json:"method,omitempty" jsonschema:"required,title=Method" jsonschema_description:"HTTP method for route.\n\nThis indicates the method which if it hits the route then the pipeline will be triggered."`
	RecordLogs *bool          `json:"recordLogs,omitempty" jsonschema:"title=Record Logs" jsonschema_description:"If set to 'true', then Appbase would record logs for the pipeline route. Defaults to 'false'."`
	Classify   *ClassifyRoute `json:"classify,omitempty" jsonschema:"title=Classify Route" jsonschema_description:"Useful to categorize the route. This is extremely useful to understand what kind of request is being sent through the route is suggested to be used in all pipeline definitions as ReactiveSearch way better when this is defined properly."`
}

type Trigger struct {
	Expression string `json:"expression,omitempty" jsonschema:"title=Trigger Expression" jsonschema_description:"Custom trigger expression. You can read more at [here](https://docs.reactivesearch.io/docs/search/rules/#advanced-editor)."`
}

// Handle storing the stage
type ESPipelineStage struct {
	Use             *PreBuiltStage `json:"use,omitempty"`
	ID              *string        `json:"id,omitempty"`
	Enabled         *bool          `json:"enabled,omitempty"`
	Async           *bool          `json:"async,omitempty"`
	Script          *string        `json:"script,omitempty"`
	ScriptRef       *string        `json:"scriptRef,omitempty"`
	ContinueOnError *bool          `json:"continueOnError,omitempty"`
	Inputs          *string        `json:"inputs,omitempty"`
	Needs           *[]string      `json:"needs,omitempty"`
	Description     *string        `json:"description,omitempty"`
	Trigger         *Trigger       `json:"trigger,omitempty"`
}

// Handle incoming stage data
type ESPipelineStageIn struct {
	Use             *PreBuiltStage          `json:"use,omitempty" jsonschema:"anyof_required=use,title=Pre-built Stage" jsonschema_description:"Use a pre-built stage from Appbase."`
	ID              *string                 `json:"id,omitempty" jsonschema:"anyof_required=id,title=Stage Id" jsonschema_description:"User-defined unique identifier for stages. It is useful to define stage dependencies using 'needs' property."`
	Enabled         *bool                   `json:"enabled,omitempty" jsonschema:"title=Enabled" jsonschema_description:"Set to 'false' to disable a stage. Defaults to 'true'."`
	Async           *bool                   `json:"async,omitempty" jsonschema:"title=Execute Asynchronously" jsonschema_description:"If set to 'true', then stage would get executed in parallel to other stages. Async stages can not modify the global 'request' and 'response' properties. Although, you can define a synchronous stage to consume the data of async stage (would be present in global context with stage id) to modify the global request/response."`
	Script          *string                 `json:"script,omitempty" jsonschema:"title=Script" jsonschema_description:"Custom script to modify the request/response. You can also write custom variables to global context which can be consumed by other stages."`
	ScriptRef       *string                 `json:"scriptRef,omitempty" jsonschema:"title=Script Reference" jsonschema_description:"Path to script file.\n\nThis is similar to _script_, except that it accepts a path to a file instead of an inline string. This is useful for cases where there is a very large JS script that needs to be used in one or more stages of a pipeline."`
	ContinueOnError *bool                   `json:"continueOnError,omitempty" jsonschema:"title=Continue on Error" jsonschema_description:"If set to 'false' and an error occurs in stage execution, then Pipeline execution would stop immediately with an error."`
	Inputs          *map[string]interface{} `json:"inputs,omitempty" jsonschema:"title=Stage Inputs" jsonschema_description:"Inputs required for a pre-built stage execution. The inputs structure may vary for each stage."`
	Needs           *[]string               `json:"needs,omitempty" jsonschema:"title=Needs" jsonschema_description:"Useful to define the dependencies among stages. For example, if stage 'A' depends on stages 'B' and 'C' then stage 'A' would define 'needs' property as ['B', 'C']. Stage 'A' would only get executed once the stages 'B' and 'C' are completed."`
	Description     *string                 `json:"description,omitempty" jsonschema:"title=Description" jsonschema_description:"User-defined description for stage."`
	Trigger         *Trigger                `json:"trigger,omitempty" jsonschema:"title=Trigger" jsonschema_description:"Trigger will indicate whether or not to trigger the stage."`
}

// ValidateResponse will return the validate endpoints
// response
type ValidateResponse struct {
	ConsoleLogs  *interface{}   `json:"console_logs,omitempty"`
	Request      *interface{}   `json:"request,omitempty"`
	Response     *interface{}   `json:"response,omitempty"`
	Envs         *interface{}   `json:"envs,omitempty"`
	Took         *int           `json:"took,omitempty"`
	StageChanges *[]interface{} `json:"stageChanges,omitempty"`
}

// Classify route details
type ClassifyRoute struct {
	Category *category.Category `json:"category,omitempty" jsonschema:"required,title=Category," jsonschema_description:"Route category.\n\nThis indicates the category of the route. This is useful for the internal functioning of the pipeline."`
	ACL      *acl.ACL           `json:"acl,omitempty" jsonschema:"title=ACL" jsonschema_description:"It a sub-category of category.\n\nThis can be thought of as narrowing down the type of route. For eg, if category is the type of fruits, sub category can be type of a particular fruit. So Apple can be considered a category and Red or Green apples can be considered ACL's of Apple."`
}

// returns the size limit by plan
func getSizeLimitByPlan() int64 {
	if util.GetTier() != nil {
		switch util.GetTier().String() {
		case util.ProductionFirst2019.String(),
			util.ProductionFirst2021.String(),
			util.ProductionFirst2023.String():
			return 100
		case util.ArcEnterprise.String(),
			util.HostedArcEnterprise.String(),
			util.HostedArcEnterprise2021.String(),
			util.ProductionSecond2019.String(),
			util.ProductionThird2019.String(),
			util.ProductionSecond2021.String(),
			util.ProductionThird2021.String(),
			util.ProductionFourth2019.String():
			return 1000
		default:
			return 10
		}
	}
	return 10
}

// IsPipelineLimitExceeded will check if the plan limit has been
// exceeded for the passed value and will accordingly return
func IsPipelineLimitExceeded(value int64) bool {
	plan := util.GetTier()
	if plan == nil {
		return value > 10
	}

	return plan.LimitForPlan().Pipelines.IsLimitExceeded(int(value))
}

// Convert an ESPipelineDoc to map[string]interface
func PipelineToMap(pipeline ESPipelineDoc) (map[string]interface{}, error) {
	parsedJson, err := json.Marshal(pipeline)
	if err != nil {
		return nil, err
	}

	var iterableBody map[string]interface{}
	err = json.Unmarshal(parsedJson, &iterableBody)
	if err != nil {
		return nil, err
	}

	return iterableBody, nil
}

// Convert a map[string]interface{} to ESPipelineDoc
func MapToPipeline(doc map[string]interface{}) (*ESPipelineDoc, error) {
	parsedJson, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	var pipeline ESPipelineDoc
	err = json.Unmarshal(parsedJson, &pipeline)
	if err != nil {
		return nil, err
	}

	return &pipeline, nil
}

// BlacklistedPaths will return an array of strings that are
// blacklisted path patterns.
func BlacklistedPaths() []string {
	return []string{
		"/_billing.*?",
	}
}

// Validate path to make sure it follows the following:
// - should start with slash (/)
func ValidatePath(path string, pathIndex int) error {
	if string([]rune(path)[0]) != "/" {
		return errors.New(fmt.Sprint("path should start with a slash (/) for route number: ", pathIndex+1))
	}

	// Make sure blacklisted paths don't match, else raise error
	for _, blacklistedPathPatter := range BlacklistedPaths() {
		match, err := regexp.MatchString(blacklistedPathPatter, path)
		if err != nil {
			return err
		}

		if match {
			return errors.New(fmt.Sprint(path, ": path not allowed"))
		}
	}

	// We will allow `index` to be passed as regex in the path
	matchIndexRe := regexp.MustCompile("^.*?{.*?}.*?$")
	if matchIndexRe.MatchString(path) {
		cleanRe := regexp.MustCompile(`^.*?{|}.*?$`)
		cleanedUpVar := cleanRe.ReplaceAllString(path, "")

		if cleanedUpVar != "index" {
			return fmt.Errorf("to match index, `{index}` should be passed in the path, instead of `{%s}`", cleanedUpVar)
		}
	}

	return nil
}

// Get valid http methods for router
func GetHttpMethods() []string {
	return []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodPost,
		http.MethodPut,
		http.MethodPatch,
		http.MethodConnect,
		http.MethodTrace,
	}
}

// StagesRequireRS returns the stages that require RS
// as category in routes
func StagesRequireRS() []PreBuiltStage {
	return []PreBuiltStage{
		ReactiveSearchQuery,
		RecordAnalytics,
	}
}

// validateCategoryIsPresent validates if the category is present
// in all routes.
func validateCategoryIsPresent(category category.Category, routes []ESPipelineRoutes) error {
	for routeIndex, route := range routes {
		if route.Classify == nil || route.Classify.Category == nil || *route.Classify.Category != category {
			return fmt.Errorf("category must be present and should be %s for route: %d", category.String(), routeIndex+1)
		}
	}
	return nil
}

// StagesRequireCategory returns a list of prebuilt stages
// that require the route category to be set
func StagesRequireCategory() []PreBuiltStage {
	return []PreBuiltStage{
		Authorization,
	}
}

// StagesAllowNeeds returns a list of prebuilt stages that does
// allow the needs property at the same stage.
func StagesAllowNeeds() []PreBuiltStage {
	return []PreBuiltStage{
		ElasticSearchQuery,
		ReactiveSearchQuery,
		SolrQuery,
	}
}

// IsValidNeedsStage checks if the passed needs stage is defined
// in the stages array
func IsValidNeedsStage(stage string, stages []ESPipelineStage, currentIndex int) bool {
	for position, definedStage := range stages {
		if position != currentIndex && definedStage.ID != nil && *definedStage.ID == stage {
			return true
		}
	}

	return false
}

// Validate methods to make sure it is one
// of the allowed ones
func ValidateMethod(method string, pathIndex int) error {
	for _, validMethod := range GetHttpMethods() {
		if validMethod == method {
			return nil
		}
	}

	return errors.New(fmt.Sprint("method should be one of: ", strings.Join(GetHttpMethods(), ", "), " for route number: ", pathIndex+1))
}

// validateStageID will make sure the ID is not used in any other stage
// except the current stage.
func validateStageID(ID string, stages []ESPipelineStage, currentIndex int, isUse bool) error {
	for stageIndex, stage := range stages {
		if stageIndex == currentIndex {
			continue
		}

		// ID will be populated before accessed here
		if stage.ID != nil && *stage.ID == ID {
			// Check if both the stages are use stages, in which case the error
			// message will be different.
			errMsg := fmt.Sprintf("ID not unique. `%s` in stage: %d reused in stage %d", ID, currentIndex+1, stageIndex+1)

			if stage.Use != nil && isUse {
				// Since both are use stages and ID was not passed, we need
				// to ask use to pass ID's
				errMsg = fmt.Sprintf("multiple stages are using `%s`, please provide unique ID's to avoid conflicts", stage.Use.String())
			}

			return errors.New(errMsg)
		}
	}

	return nil
}

// ValidateStages will validate the passed stages for the pipeline
func ValidateStages(stages []ESPipelineStage, routes []ESPipelineRoutes) error {
	// If uses is passed, all other details will be ignored.
	for position, stage := range stages {
		// Make sure either Use of ID is passed
		if stage.Use == nil && stage.ID == nil {
			return errors.New(fmt.Sprint("One of `use` or `ID` is required for stage: ", position+1))
		}

		// Make sure passed ID is not used in any other stage
		stageIdErr := validateStageID(*stage.ID, stages, position, stage.Use != nil)
		if stageIdErr != nil {
			return stageIdErr
		}

		// Make sure ID and use are not passed together
		// We auto generate an ID for use case, in this case, it's not an
		// use passed ID. If the user passes an ID, we do not overwrite it
		// with auto generated ID so that.
		//
		// if stage.Use != nil && stage.ID != nil {
		// 	return errors.New(fmt.Sprint("`ID` and `use` cannot be passed at the same time for stage: ", position+1))
		// }

		// Make script a require parameter if ID is passed and it's not `use`
		if stage.ID != nil && stage.Use == nil && stage.Script == nil {
			return errors.New(fmt.Sprint("`script` is a required parameter, can't be empty for stage: ", position+1))
		}

		// Check if `needs` is valid
		if stage.Needs != nil {
			for _, needStage := range *stage.Needs {
				if !IsValidNeedsStage(needStage, stages, position) {
					return errors.New(fmt.Sprintf("`%s` not a valid needs stage for stage: %d", needStage, position+1))
				}
			}
		}

		// If stage is use and one of those that require RS as category
		if stage.Use != nil {
			isRSCategoryRequired := false
			for _, prebuiltStage := range StagesRequireRS() {
				if *stage.Use == prebuiltStage {
					isRSCategoryRequired = true
					break
				}
			}

			if isRSCategoryRequired {
				// Make sure RS category is set in all paths
				err := validateCategoryIsPresent(category.ReactiveSearch, routes)
				if err != nil {
					return err
				}
			}

			// Validate if category is absolutely required
			isCategoryRequired := false
			for _, categoryRequiredStage := range StagesRequireCategory() {
				if *stage.Use == categoryRequiredStage {
					isCategoryRequired = true
					break
				}
			}

			if isCategoryRequired {
				// Make sure all routes have a category defined in them
				for routeIndex, route := range routes {
					if route.Classify == nil || route.Classify.Category == nil {
						return errors.New(fmt.Sprintf("category is required at route: %d", routeIndex+1))
					}
				}
			}

			// Validate needs allowed for stage
			if stage.Needs != nil {
				isDenied := true
				for _, allowNeedStage := range StagesAllowNeeds() {
					if allowNeedStage == *stage.Use {
						isDenied = false
						break
					}
				}

				if isDenied {
					return errors.New(fmt.Sprintf("needs is not allowed when use is set to `%s`", stage.Use.String()))
				}
			}
		}

		// If stage has trigger present, validate the trigger expression
		// to make sure it doesn't have any syntax errors.
		if stage.Trigger != nil {
			stageTriggerErr := validateStageTrigger(stage.Trigger.Expression)
			if stageTriggerErr != nil {
				return errors.New(fmt.Sprintf("trigger expression is invalid for stage: %d with err: %v", position+1, stageTriggerErr))
			}
		}
	}

	// Validate cyclic dependency
	_, err := getStageStatusMap(stages)
	if err != nil {
		return err
	}

	return nil
}

// parsePipelineRefFile should parse file using the passed key
// from the request passed and return a string.
func parsePipelineRefFile(req *http.Request, scriptRef string, stageIndex int) (string, error) {
	// Find the file using the passed key
	refFile, refFileDetails, err := req.FormFile(scriptRef)
	if err != nil {
		// Try to read the file content as a JSON.
		refFileStr := req.PostFormValue(scriptRef)
		if refFileStr == "" {
			// It's neither a file nor a valid string.
			return "", errors.New(fmt.Sprint(scriptRef, ": value passed is not a file neither a valid string for stage: ", stageIndex+1))
		}

		// Else parse the content
		var scriptRefContent Config
		err := json.Unmarshal([]byte(refFileStr), &scriptRefContent)
		if err != nil {
			return "", errors.New(fmt.Sprint(scriptRef, ": invalid JSON content passed for stage: ", stageIndex+1))
		}

		// If the marshal happened properly, validate the type to be JS if it is not nil
		if scriptRefContent.Content == nil || scriptRefContent.Type == nil {
			return "", errors.New(fmt.Sprint(scriptRef, ": content and extension are required fields for stage: ", stageIndex+1))
		}

		// Validate the extension
		if strings.ToLower(*scriptRefContent.Type) != "js" {
			return "", errors.New(fmt.Sprint("non JS file passed for scriptRef at stage: ", stageIndex+1))
		}

		// Finally return the content
		return *scriptRefContent.Content, nil
	}

	// Verify the extension to make sure it's a JS file.
	if strings.ToLower(filepath.Ext(refFileDetails.Filename)) != ".js" {
		return "", errors.New(fmt.Sprint("non JS file passed for scriptRef at stage: ", stageIndex+1))
	}

	// Try to read the file now
	var fileContent = make([]byte, refFileDetails.Size)
	_, readErr := refFile.Read(fileContent)
	if readErr != nil {
		return "", errors.New(fmt.Sprint("failed while reading scriptRef file for stage: ", stageIndex+1))
	}

	return string(fileContent), nil
}

// convertPipelineInToES converts the pipeline body
// passed to make it storable in ES
//
// originalConfig will have to be populated by the parent
// and this method will not do anything to that.
func convertPipelineInToES(pipelineIn ESPipelineDocIn) (ESPipelineDoc, error) {
	var stages []ESPipelineStage
	var routes []ESPipelineRoutes
	var esPipelineDoc = ESPipelineDoc{
		ID:              pipelineIn.ID,
		Description:     pipelineIn.Description,
		Enabled:         pipelineIn.Enabled,
		Priority:        pipelineIn.Priority,
		Envs:            pipelineIn.Envs,
		Trigger:         pipelineIn.Trigger,
		UpdatedAt:       pipelineIn.updatedAt,
		CreatedAt:       pipelineIn.createdAt,
		ValidateContext: pipelineIn.ValidateContext,
	}

	// Make sure stages are not empty
	if pipelineIn.Stages == nil || pipelineIn.Routes == nil {
		return esPipelineDoc, nil
	}

	for _, route := range *pipelineIn.Routes {
		defaultEnabled := true
		if route.RecordLogs == nil {
			route.RecordLogs = &defaultEnabled
		}

		routes = append(routes, route)
	}

	for _, stage := range *pipelineIn.Stages {
		// If script is passed and is not an empty string
		// convert it to binary
		if stage.Script != nil && *stage.Script != "" {
			binScript := base64.StdEncoding.EncodeToString([]byte(*stage.Script))
			stage.Script = &binScript
		}

		// If `use` is passed, set the stage ID the value of pre-built stage name
		if stage.Use != nil && stage.ID == nil {
			id := stage.Use.String()
			stage.ID = &id
		}

		// If `continueOnError` is not passed, set it to true
		defaultTrue := true
		if stage.ContinueOnError == nil {
			stage.ContinueOnError = &defaultTrue
		}

		// convert the incoming stage to the stage to be
		// stored
		stageForES, err := convertStageInToES(stage)
		if err != nil {
			log.Warnln(logTag, ": error while converting input stage to ES: ", err)
			return esPipelineDoc, err
		}

		// Append the stage
		stages = append(stages, stageForES)
	}

	// Put the stages in the esPipelineDoc
	esPipelineDoc.Stages = &stages

	// Put the routes in the es pipeline
	esPipelineDoc.Routes = &routes

	return esPipelineDoc, nil
}

// convertStageInToES will convert the incoming stage to make
// it storable on ES.
func convertStageInToES(stageIn ESPipelineStageIn) (ESPipelineStage, error) {
	var esStage = ESPipelineStage{
		Use:             stageIn.Use,
		ID:              stageIn.ID,
		Enabled:         stageIn.Enabled,
		Async:           stageIn.Async,
		Script:          stageIn.Script,
		ScriptRef:       stageIn.ScriptRef,
		ContinueOnError: stageIn.ContinueOnError,
		Needs:           stageIn.Needs,
		Description:     stageIn.Description,
		Trigger:         stageIn.Trigger,
	}

	// Convert the inputs to a string to make it
	// storable
	if stageIn.Inputs != nil {
		inputsInBytes, err := json.Marshal(*stageIn.Inputs)
		if err != nil {
			return esStage, err
		}

		stringifiedInputs := string(inputsInBytes)
		esStage.Inputs = &stringifiedInputs
	}

	return esStage, nil
}

// decodePipelineScripts should decode the scripts in the pipeline and return
// a ES pipeline body with proper string in scripts.
func decodePipelineScripts(encodedDoc ESPipelineDoc) ESPipelineDoc {
	// Since encoded body is validated, we can directly iterate the
	// stages.
	for stageIndex, stage := range *encodedDoc.Stages {
		if stage.Script == nil || *stage.Script == "" {
			continue
		}

		var decodedScriptStr string
		decodedScript, err := base64.StdEncoding.DecodeString(*stage.Script)
		// if string isn't base64, no need to decode it
		if err != nil {
			decodedScriptStr = *stage.Script
		} else {
			decodedScriptStr = string(decodedScript)
		}
		stage.Script = &decodedScriptStr
		(*encodedDoc.Stages)[stageIndex] = stage
	}

	return encodedDoc
}

// addInitVersionIfNotPresent will add the first version of the pipeline
// if the versions array is empty.
//
// NOTE: This function should get the scripts decoded into strings.
func addInitVersionIfNotPresent(originalDoc ESPipelineDoc) ESPipelineDoc {
	if originalDoc.Versions != nil {
		return originalDoc
	}

	// Since versions is not present, add a first version here.
	versionsArr := make([]Version, 0)
	defaultLiveVersion := 1
	defaultVersionDesc := "Init pipeline version"

	// If the body is valid, update the updatedAt date for the pipeline and set the new version
	versionAsBytes, _ := json.Marshal(originalDoc)

	versionAsStr := string(versionAsBytes)

	versionsArr = append(versionsArr, Version{
		Content:     &versionAsStr,
		Version:     &defaultLiveVersion,
		Description: &defaultVersionDesc,
	})
	originalDoc.LiveVersion = &defaultLiveVersion
	originalDoc.Versions = &versionsArr

	return originalDoc
}

// getConfigOut generates the configOut from the passed
// pipeline body.
//
// pipeline should be the ESPipelineDoc that will be parsed
// to ConfigOut.
//
// parseTopLevelPipeline should indicate whether or not the
// top level of the pipeline is being parsed.
// This flag will be true only if the pipeline passed is the
// top level pipeline fetched from ES directly and not nested
// versions of pipeline.
func getConfigOut(pipeline ESPipelineDoc, parseTopLevelPipeline bool) ConfigOut {
	// Just in case original config is nil, make sure it doesn't
	// crash the server
	var original = new(Config)
	var emptyStr = ""

	if pipeline.OriginalConfig == nil {
		original.Content = &emptyStr
		original.Type = &emptyStr

		// Set the pipeline.originalconfig
		pipeline.OriginalConfig = original
	}

	versionDescription := ""

	// Set the default version as 1
	defaultVersion := 1
	versionLive := &defaultVersion

	// Make sure that the pipeline is the top level that is being
	// parsed and the LiveVersion field is present here.
	if parseTopLevelPipeline && pipeline.LiveVersion != nil {
		// Extract the version description for the live version
		// of the pipeline passed.
		//
		// We will also extract the version content and unmarshal
		// it into ESPipelineDoc so that the pipeline details returned
		// are always the live pipeline.
		versionContent, _ := GetPipelineVersion(pipeline, *pipeline.LiveVersion)

		// Just a legacy check but there might be a case where
		// versionContent is nil because the live version is not found.
		//
		// In such a case, make the current pipeline the live version.
		if versionContent != nil {
			versionDescription = *versionContent.Description
			versionLive = versionContent.Version

			// Parse the content into an ESPipelineDoc and update
			// the pipeline var with it.
			var versionPipeline ESPipelineDoc
			json.Unmarshal([]byte(*versionContent.Content), &versionPipeline)

			pipeline = versionPipeline
		}
	}

	routes := make([]ESPipelineRoutes, 0)

	// Populate the routes
	if pipeline.Routes != nil {
		routes = append(routes, *pipeline.Routes...)
	}

	configOut := ConfigOut{
		ID:              pipeline.ID,
		Content:         pipeline.OriginalConfig.Content,
		Type:            pipeline.OriginalConfig.Type,
		Enabled:         pipeline.Enabled,
		Priority:        pipeline.Priority,
		Description:     pipeline.Description,
		UpdatedAt:       pipeline.UpdatedAt,
		CreatedAt:       pipeline.CreatedAt,
		Routes:          routes,
		ValidateContext: pipeline.ValidateContext,
	}

	if parseTopLevelPipeline {
		configOut.Version = versionLive
		configOut.VersionDescription = &versionDescription
	}

	return configOut
}

// runTrigger runs the trigger for the pipeline and checks if it
// returned true or false.
func runTrigger(ctx context.Context, r *http.Request, pipeline ESPipelineDoc) (bool, error) {
	if pipeline.Trigger != nil && pipeline.Trigger.Type != nil {
		triggerType := pipeline.Trigger.Type
		if *triggerType == rules.Filter {
			// Since it is a filter request, the request must be RSQuery, extract it from the
			// context
			//
			// Since this request is a pipeline one, the RSQuery won't be available in the ctx,
			// we will have to extract the current request body and try to convert it to RSQuery
			// Extract the body
			var req *querytranslate.RSQuery

			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(r.Body)

			// Replace original body with the same body
			// since it was emptied when we read it.
			r.Body = ioutil.NopCloser(buf)

			// Try to onvert body to RSQuery
			marshalErr := json.Unmarshal(buf.Bytes(), &req)
			if marshalErr != nil {
				log.Warnln(logTag, " error while unmarshalling passed body to RSQuery for filter validation, ", err)
				return false, marshalErr
			}

			environments := querytranslate.ExtractEnvsFromRequest(*req)

			parsedEnvironments, err := rules.GetTriggerEnvs(ctx, r, environments, *pipeline.Trigger.Type)
			if err != nil {
				return false, err
			}

			// Run for every query in the array
			for _, v := range req.Query {
				parsedEnvironments.Type = v.Type.String()
				program, err := expr.Compile(rules.ParseTriggerExpression(rules.ParseTriggerExprToLowerCase(pipeline.Trigger.Expression), *triggerType), expr.Env(parsedEnvironments), expr.AsBool())
				if err != nil {
					return false, err
				}
				output, err := expr.Run(program, *parsedEnvironments)
				if err != nil {
					return false, err
				}
				if output.(bool) {
					return true, nil
				}
			}
			return false, err
		} else if *triggerType == rules.Index {
			// If expression is not passed, then validate based on the ACL and
			// Category to see if it is an indexing request
			if pipeline.Trigger.Expression == "" {
				return rules.IsIndexingRequest(r), nil
			}

			// This is for indexing requests
			var environments querytranslate.QueryEnvs

			parsedEnvironments, err := rules.GetTriggerEnvs(ctx, r, environments, *pipeline.Trigger.Type)
			if err != nil {
				return false, err
			}

			program, err := expr.Compile(rules.ParseTriggerExpression(rules.ParseTriggerExprToLowerCase(pipeline.Trigger.Expression), *triggerType), expr.Env(parsedEnvironments), expr.AsBool())
			if err != nil {
				return false, err
			}
			output, err := expr.Run(program, *parsedEnvironments)
			if err != nil {
				return false, err
			}
			if output.(bool) {
				return true, nil
			}
		}
	}

	return true, nil
}

// validateTimeFrame validates the timeframe for the trigger if any
// passed.
func validateTimeframe(pipeline ESPipelineDoc) bool {
	if pipeline.Trigger != nil && pipeline.Trigger.TimeFrame != nil {
		currentTime := time.Now().Unix()
		// validate start time
		if pipeline.Trigger.TimeFrame.StartTime != nil {
			if currentTime < *pipeline.Trigger.TimeFrame.StartTime {
				return false
			}
		}
		// validate end time
		if pipeline.Trigger.TimeFrame.EndTime != nil {
			if currentTime > *pipeline.Trigger.TimeFrame.EndTime {
				return false
			}
		}
	}
	return true
}

// getStageContextDiff will find the diff between the
// passed stage contexts and return it
func getStageContextDiff(oldContext []byte, newContext []byte) string {
	dmp := diffmatchpatch.New()
	headerDiff := dmp.DiffMain(string(oldContext), string(newContext), false)
	return dmp.DiffToDelta(headerDiff)
}

func GetPipelineSchema() ([]byte, error) {
	schema := GetReflactor().Reflect(&ESPipelineDocIn{})
	schemaMarshalled, marshalErr := schema.MarshalJSON()

	if marshalErr != nil {
		return nil, marshalErr
	}

	return injectExtrasToSchema(schemaMarshalled, *schema)
}

var jsonSchemaInstance *jsonschema.Reflector
var jsonSchemaInstanceOnce sync.Once

func GetReflactor() *jsonschema.Reflector {
	jsonSchemaInstanceOnce.Do(func() {
		r := new(jsonschema.Reflector)
		r.ExpandedStruct = true
		r.AllowAdditionalProperties = true
		r.DoNotReference = true
		r.RequiredFromJSONSchemaTags = true
		jsonSchemaInstance = r
	})
	return jsonSchemaInstance
}

// createPipelineSchema will create the pipeline schema in
// the current directory's schema/latest/pipelines-schema.json
func createPipelineSchema() error {
	pathToCreate := filepath.Join("schema", "latest")
	dirCreateErr := os.MkdirAll(pathToCreate, os.ModePerm)

	if dirCreateErr != nil {
		return dirCreateErr
	}

	// Since the directory is created, write the contents into the file
	// now.
	schemaContent, schemaErr := GetPipelineSchema()

	if schemaErr != nil {
		return schemaErr
	}

	// Unmarshal the json into a map and then marshal with indent to make it look
	// better.
	tempMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(schemaContent, &tempMap)
	if unmarshalErr != nil {
		return fmt.Errorf("error while unmarshalling bytes schema to indent it before writing to file: %v", unmarshalErr)
	}

	// Now marshal back but with indent
	writableBytes, marshalErr := json.MarshalIndent(tempMap, "", "  ")
	if marshalErr != nil {
		return fmt.Errorf("error while marshalling schema with indent to properly write it to file")
	}

	// Finally write the content into a file
	return ioutil.WriteFile(filepath.Join(pathToCreate, "pipelines-schema.json"), writableBytes, 0644)
}

// injectExtrasToSchema will inject extra values to the schema like
// description.
func injectExtrasToSchema(schemaMarshalled []byte, originalSchema jsonschema.Schema) ([]byte, error) {
	schemaAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(schemaMarshalled, &schemaAsMap)

	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	// Update the properties with injection of values.
	propertiesAsMap := schemaAsMap["properties"].(map[string]interface{})

	// Iterate and inject the descriptions wherever necessary.
	iterateAndInject(&propertiesAsMap)

	schemaAsMap["properties"] = propertiesAsMap

	// Inject a top-level preservedOrder field as well
	schemaAsMap["preservedOrder"] = originalSchema.Properties.Keys()

	return json.Marshal(schemaAsMap)
}

func iterateAndInject(propertyMap *map[string]interface{}) {
	for propKey, propValue := range *propertyMap {
		log.Debug(logTag, ": propKey: ", propKey)
		propValueAsMap, asMapOk := propValue.(map[string]interface{})
		if !asMapOk {
			log.Debug("Value is not a map, skipping with value: ", pretty.Formatter(propValue))
			continue
		}

		// If items.properties exists, then recurse into that level
		// to substitute
		if propValueAsMap["items"] != nil {
			itemsAsMap, asMapOk := propValueAsMap["items"].(map[string]interface{})
			if asMapOk && itemsAsMap["properties"] != nil {
				propertiesAsMap, propAsMapOk := itemsAsMap["properties"].(map[string]interface{})
				if propAsMapOk {
					iterateAndInject(&propertiesAsMap)

					// Once injected, update the values
					itemsAsMap["properties"] = propertiesAsMap
					propValueAsMap["items"] = itemsAsMap
				}
			}
		} else if propValueAsMap["stages"] != nil {
			stagesAsMap, asMapOk := propValueAsMap["stages"].(map[string]interface{})
			if asMapOk {
				iterateAndInject(&stagesAsMap)

				// Once injected, update the values
				propValueAsMap["stages"] = stagesAsMap
			}
		} else if propValueAsMap["inputs"] != nil {
			itemsAsMap, asMapOk := propValueAsMap["inputs"].(map[string]interface{})
			if asMapOk && itemsAsMap["properties"] != nil {
				propertiesAsMap, propAsMapOk := itemsAsMap["properties"].(map[string]interface{})
				if propAsMapOk {
					iterateAndInject(&propertiesAsMap)

					// Once injected, update the values
					itemsAsMap["properties"] = propertiesAsMap
					propValueAsMap["inputs"] = itemsAsMap
				}
			}
		}

		// If properties exists, then recurse into that level as well to
		// substitute
		if propValueAsMap["properties"] != nil {
			propertiesAsMap, asMapOk := propValueAsMap["properties"].(map[string]interface{})
			if asMapOk {
				iterateAndInject(&propertiesAsMap)
				propValueAsMap["properties"] = propertiesAsMap
			}
		}

		// Substitute description if it is empty
		oldDesc, descOk := propValueAsMap["description"]
		log.Debug(logTag, ": desc: ", oldDesc, ": descOk: ", descOk)
		if !descOk || oldDesc == "" {
			descToPut, isPresent := MARKDOWN_DESCRIPTION[propKey]
			if isPresent {
				propValueAsMap["description"] = descToPut
				log.Debug(logTag, ": Updated desc to: ", descToPut)
			}
		}

		(*propertyMap)[propKey] = propValueAsMap
	}
}

var MARKDOWN_DESCRIPTION = map[string]string{
	"routes":          "**This is a required field**\n\nPipeline routes.\n\nRoutes is an array of route which essentially indicates which routes the pipeline will be listening to. In other words, which routes will trigger the pipeline can be defined using this field.\n\nFollowing is an example of routes:\n\n```yml\nroutes:\n  - path: good-books-ds-pipeline/_reactivesearch\n    method: POST\n    classify:\n      category: reactivesearch\n```\n\nAbove code indicates that the pipeline will be triggered if the route is `good-books-ds-pipeline/_reactivesearch` and the method is `POST`.",
	"trigger":         "Trigger expression is to define the condition of Pipeline invocation. For example, only execute pipeline if query is \\'mobile phone\\'. Check the documentation at [here](https://docs.reactivesearch.io/docs/search/rules/#configure-if-condition).\n\nFollowing is an example trigger for a pipeline that searches for mobile phones:\n\n```yml\ntrigger:\n  type: always\n  expression: $query exactlyMatches \"iphone x\"\n```\n\nAbove trigger will **always** run and execute the expression provided to it.",
	"stages":          "**This is a required field**\n\nPipeline stages.\n\nStages can be thought of as steps of the pipeline that are executed (not always in the order of specification).\n\nFollowing is an example of pipeline stages:\n\n```yml\n\nstages:\n  - use: authorization\n  - use: useCache\n  - id: echo something\n    script: \"console.log(\\'Echoing something from a JS script instead of shell!\\');\"\n  - use: reactivesearchQuery\n    continueOnError: false\n  - use: elasticsearchQuery\n    continueOnError: false\n  - use: recordAnalytics\n```\n\nAbove uses some pre-built stages as well as a stage where the `script` field is used to just run some custom JavaScript. More can be read about the pre-built stages in the following section.",
	"url":             "The `url` field is used to specify the URL that is supposed to be hit during validating the global environment before adding it.",
	"method":          "It might be important to specify the method field in order to get the `expected_status`. This can be done by passing the method as a string. By default the value is set to `GET`.\n\nSome of the other valid options are:\n\n- `POST`\n- `PUT`\n- `PATCH`",
	"body":            "At times, there might be the need to pass the body in a response in order to get the `expected_status`. This is also supported by passing the body in the `body` field.\n\nThe body should be passed as a **string**. If JSON, this should be a stringified JSON.",
	"headers":         "Headers can be essential to alter the response received from hitting a particular URL. Headers can be passed during validating by using the `headers` field.\n\nFor eg, a `Content-Type` header can be passed in the following way:\n\n```yml\nglobal_envs:\n  - label: ES URL\n    key: ES_URL\n    value: http://localhost:9200\n    validate:\n      headers:\n        \"Content-Type\": \"application/json\"\n```",
	"expected_status": "The `expected_status` field is used to make sure the validation was successful. It is an integer that should match the status code of the validate request when it is successful.",
	"value":           "To define the matching values to boost results, for e.g show items with '['holiday-sale', 'premium values']' for 'tag' data field above other items. This key also supports other types of values in order to support `range` and `geo` type of queries.\n\nTo use with `geo` for boosting, value can be ```yaml\nlocation: '23.7555,76.4444'\ndistance: 100\nunit:'km'\n```\n\nTo use with `range` for boosting, value can be ```yaml\nstart: 23\nend: 45\n```",
}

// validatePipelineVar will validate the pipeline var passed
// in the request
func validatePipelineVar(varPassed PipelineVar, existsOk bool) error {
	// Make sure that key and value are passed
	if varPassed.Key == nil || varPassed.Value == nil {
		return errors.New("`key` and `value` are required fields")
	}

	// Check if Key already exists
	if !existsOk {
		_, location := GetVarAndLocation(*varPassed.Key)
		if location != nil {
			return errors.New(fmt.Sprintf("%s: is already present", *varPassed.Key))
		}
	}

	return nil
}

// parseRangeValue will parse the interface range
// value to string.
//
// First attempt will be to parse to number and
// then to string, else directly to string, and finally
// error out.
func parseRangeValue(value interface{}) (string, error) {
	valueAsInt, intOk := value.(float64)
	if intOk {
		return fmt.Sprint(valueAsInt), nil
	}

	// Try to convert to string directly.
	valueAsStr, strOk := value.(string)
	if !strOk {
		return "", errors.New("invalid type passed for range field")
	}

	// If range is of type date and timezone is not passed
	// then append it.
	doesDateMatch, matchErr := regexp.MatchString("^[0-9]{4}-[0-9]{2}-[0-9]{2}$", valueAsStr)
	if matchErr == nil && doesDateMatch {
		valueAsStr += "T00:00:00Z"
	}

	// Else throw error since invalid value.
	return valueAsStr, nil
}

// checkAggregationValues will Check what values are present in
// the aggregations array and accordingly return a response.
//
// Following values are returned:
// First value will indicate whether or not min is present.
// Second value will indicate whether or not max is present.
// Third value will indicate whether or not histogram is present.
func checkAggregationValues(aggregations []string) (bool, bool, bool) {
	isMinPresent := false
	isMaxPresent := false
	isHistogramPresent := false

	for _, aggregationValue := range aggregations {
		// If both flags are true, no need to keep iterating
		// since we already have the final result.
		if isMinPresent && isMaxPresent && isHistogramPresent {
			break
		}

		if aggregationValue == "min" {
			isMinPresent = true
			continue
		}

		if aggregationValue == "max" {
			isMaxPresent = true
			continue
		}

		if aggregationValue == "histogram" {
			isHistogramPresent = true
		}
	}

	return isMinPresent, isMaxPresent, isHistogramPresent
}

// convertDistanceForUnit will convert the distance based
// on the current unit to `km`. This is required in Solr
// support for geo type of queries.
func convertDistanceForUnit(distance float64, convertFrom string) (float64, error) {
	// We need to convert the unit passed as string
	// into the libraries unit type.
	unitMap := map[string]units.Unit{
		"km": units.KiloMeter,
		"mi": units.Mile,
		"yd": units.Yard,
		"ft": units.Foot,
		"m":  units.Meter,
		"cm": units.CentiMeter,
		"nm": units.NanoMeter,
		// NOTE: Following is just to make sure nmi is a valid
		// format.
		"nmi": units.Mile,
	}

	// Throw an error if the distance is negative
	if math.Signbit(distance) {
		return distance, fmt.Errorf("invalid `value.distance` field passed, cannot accept negative values as distance")
	}

	convertFrom = strings.ToLower(convertFrom)

	mappedUnit, unitPresent := unitMap[convertFrom]
	if !unitPresent {
		// Invalid unit passed.
		return distance, fmt.Errorf("invalid unit `%s` passed in geo query", convertFrom)
	}

	// If the convertFrom is nmi, do it manually since the library doesn't
	// support it.
	if convertFrom == "nmi" {
		// 1 km is equal to 1.852 nautical miles so we need to
		// multiply the passed nautical mile by that to get the
		// distance in km's
		return distance * 1.852, nil
	}

	// If it is km, we don't need to convert at all, so the better option would be
	// to return the value as is instead.
	if convertFrom == "km" {
		return distance, nil
	}

	// If the match happened, convert the distance to km
	convertedDistance, convertErr := units.ConvertFloat(distance, mappedUnit, units.KiloMeter)

	if convertErr != nil {
		return distance, fmt.Errorf("error while converting distance from passed unit, %v", convertErr)
	}

	return convertedDistance.Float(), nil
}

// validateStageTrigger will validate the trigger expression
// to make sure the syntax is valid.
func validateStageTrigger(expression string) error {
	triggerEnvs := map[string]interface{}{
		"envs":    map[string]interface{}{},
		"inputs":  map[string]interface{}{},
		"context": map[string]interface{}{},
	}

	_, err := expr.Compile(rules.ParseTriggerExprToLowerCase(expression), expr.Env(triggerEnvs), expr.AsBool())
	return err
}

// runStageTrigger will run the stage's trigger by injecting various values
// regarding the pipeline and accordingly return true or false depending
// on the result received from expr.
//
// The bool returned will indicate whether the trigger evaluated
// to true or false.
//
// If the evaluated bool is `true` the stage should be executed, else
// the stage will be skipped.
func runStageTrigger(pipeline ESPipelineDoc, executionCtx PipelineExecutionContext, stageId string, globalCtx []byte) (bool, error) {
	stageIndex := getStageIndex(stageId, pipeline)
	if stageIndex == -1 {
		errToReturn := fmt.Errorf("error while getting stage index, no stage found with ID: %s", stageId)
		log.Warnln(logTag, ": ", errToReturn)
		return false, errToReturn
	}

	stageToWorkOn := (*pipeline.Stages)[stageIndex]

	// If trigger is not passed at all, we need to consider
	// the trigger as success.
	if stageToWorkOn.Trigger == nil {
		return true, nil
	}

	// Build the generic env and then inject a few stage specific
	// values like inputs and envs if any.
	globalEnv := buildTriggerEnv(pipeline, executionCtx, globalCtx)

	if stageToWorkOn.Inputs == nil {
		defaultInputs := "{}"
		stageToWorkOn.Inputs = &defaultInputs
	}

	// Unmarshal inputs into a map
	inputsAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal([]byte(*stageToWorkOn.Inputs), &inputsAsMap)
	if unmarshalErr != nil {
		errMsg := fmt.Sprintf("error while marshalling inputs for trigger environment for stage: %d", stageIndex)
		log.Warnln(logTag, ": ", errMsg)
		return false, fmt.Errorf(errMsg)
	}

	globalEnv["inputs"] = inputsAsMap

	// Run the trigger expression
	exprToRun, compileErr := expr.Compile(rules.ParseTriggerExprToLowerCase(stageToWorkOn.Trigger.Expression), expr.Env(globalEnv), expr.AsBool())
	if compileErr != nil {
		errMsg := fmt.Errorf("error while compiling expression to run the trigger for stage: %d with err: %v", stageIndex, compileErr)
		log.Warnln(logTag, ": ", errMsg)
		return false, errMsg
	}

	output, runErr := expr.Run(exprToRun, globalEnv)
	if runErr != nil {
		errToRet := fmt.Errorf("error while running expression for stage: %d and error: %v", stageIndex, runErr)
		log.Warnln(logTag, ": ", errToRet)
		return false, errToRet
	}

	outputAsBool := output.(bool)
	return outputAsBool, nil
}

// buildTriggerEnv will build the environment for trigger so that the
// expression can be evaluated properly.
func buildTriggerEnv(pipeline ESPipelineDoc, executionCtx PipelineExecutionContext, globalCtx []byte) map[string]interface{} {
	triggerEnv := make(map[string]interface{})

	triggerEnv["envs"] = executionCtx.envs

	// Verify that body is passed as an object and not as a string

	// Parse the body into an interface
	bodyAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(executionCtx.request.Body, &bodyAsMap)

	log.Debugln(logTag, ": unmarshal error for body to interface: ", unmarshalErr)

	triggerEnv["envs"].(map[string]interface{})["body"] = bodyAsMap["body"]

	// Parse the context as well
	ctxAsInterface := new(interface{})
	ctxUnmarshalErr := json.Unmarshal(globalCtx, &ctxAsInterface)

	triggerEnv["context"] = *ctxAsInterface

	log.Debugln(logTag, ": unmarshal error for ctx to interface: ", ctxUnmarshalErr)

	return triggerEnv
}

// calculateOppositeCoordinates will calculate the coordinates
// opposite to the passed one.
//
// Expected values are topLeft and bottomRight and the return values
// will be bottomLeft and topRight.
// This is necessary to support Solr bounding box properly.
func calculateOppositeCoordinates(topLeft string, bottomRight string) (string, string, *Error) {
	// Extract the coordinates into floats first.
	yTopLeft, xTopLeft, topLeftErr := extractCoordinatesFromString(topLeft)
	if topLeftErr != nil {
		return topLeft, bottomRight, topLeftErr
	}

	yBottomRight, xBottomRight, bottomRightErr := extractCoordinatesFromString(bottomRight)
	if bottomRightErr != nil {
		return topLeft, bottomRight, bottomRightErr
	}

	bottomLeft := strings.Join([]string{fmt.Sprintf("%f", yBottomRight), fmt.Sprintf("%f", xTopLeft)}, ",")
	topRight := strings.Join([]string{fmt.Sprintf("%f", yTopLeft), fmt.Sprintf("%f", xBottomRight)}, ",")

	return bottomLeft, topRight, nil
}

// extractCoordinatesFromString will extract the coordinates from string
// to float.
//
// The string should be in the form of `x.xxx,y.yyy`.
func extractCoordinatesFromString(coordinatesAsString string) (float64, float64, *Error) {
	// Replace whitespace if any.
	whitespaceRemoved := strings.Replace(coordinatesAsString, " ", "", -1)

	splittedCoordinates := strings.Split(whitespaceRemoved, ",")
	if len(splittedCoordinates) != 2 {
		// Throw an error since only 2D coordinates are supported.
		return 0, 0, &Error{
			Err:  fmt.Errorf("Invalid value passed for coordinates: %s", coordinatesAsString),
			Code: http.StatusBadRequest,
		}
	}

	xCoordinate := splittedCoordinates[0]
	yCoordinate := splittedCoordinates[1]

	xAsFloat, xErr := strconv.ParseFloat(xCoordinate, 64)
	if xErr != nil {
		return 0, 0, &Error{
			Err:  fmt.Errorf("Error while parsing the x-coordinate for: %s with error: %v", coordinatesAsString, xErr),
			Code: http.StatusBadRequest,
		}
	}

	yAsFloat, yErr := strconv.ParseFloat(yCoordinate, 64)
	if yErr != nil {
		return 0, 0, &Error{
			Err:  fmt.Errorf("Error while parsing the y-coordinate for: %s with error: %v", coordinatesAsString, yErr),
			Code: http.StatusBadRequest,
		}
	}

	return xAsFloat, yAsFloat, nil
}

// getNextVersion will get the next version for the pipeline and accordingly
// return it.
//
// Version is determined based on the length of the current pipelines
// `version` array. This can be reliable because versions are not deleted for
// a pipeline.
func getNextVersion(pipelineDoc ESPipelineDoc) int {
	// NOTE: Not likely but check if the `versions` array is empty.

	if pipelineDoc.Versions == nil {
		return 1
	}

	return len(*pipelineDoc.Versions) + 1
}

// generateValidateId will generate the id for validation
func generateValidateId() string {
	return shortuuid.New()
}

// DelayedStageLogTracker will allow accessing stage logs for stages
// that have a background script for up-to 30 mins since it is populated.
type DelayedStageLogTracker struct {
	IsReady   bool
	RemoveAt  *int64
	logs      *StageLogTracker
	timeTaken *map[string]*int
}

func (d *DelayedStageLogTracker) GetIsReady() bool {
	return d.IsReady
}

func (d *DelayedStageLogTracker) ShouldRemove() bool {
	return d.RemoveAt != nil && *d.RemoveAt <= time.Now().Unix()
}

func (d *DelayedStageLogTracker) GetLogs() *StageLogTracker {
	return d.logs
}

func (d *DelayedStageLogTracker) GetTimeTaken() *map[string]*int {
	return d.timeTaken
}

// ValidateIDToConsoleLogs will contain the console logs against
// the validateId.
//
// This will only store the logs in case a validate request was made
// with at-least one background script.
type ValidateIDToConsoleLogs struct {
	mu         sync.Mutex
	storageMap map[string]*DelayedStageLogTracker
}

func (v *ValidateIDToConsoleLogs) Register(validateId string) *DelayedStageLogTracker {
	newLogTracker := &DelayedStageLogTracker{
		IsReady: false,
	}

	v.mu.Lock()
	v.storageMap[validateId] = newLogTracker
	v.mu.Unlock()

	return newLogTracker
}

func (v *ValidateIDToConsoleLogs) AddLogs(validateId string, logs *StageLogTracker, timeTaken *map[string]*int) {
	v.mu.Lock()
	defer v.mu.Unlock()
	removeAt := time.Now().Add(30 * time.Minute).Unix()
	logTracker, isPresent := v.storageMap[validateId]
	if !isPresent {
		logTracker = &DelayedStageLogTracker{}
	}

	logTracker.RemoveAt = &removeAt
	logTracker.logs = logs
	logTracker.timeTaken = timeTaken
	logTracker.IsReady = true
}

func (v *ValidateIDToConsoleLogs) GetLogs(validateId string) *DelayedStageLogTracker {
	v.mu.Lock()
	defer v.mu.Unlock()
	logs, isPresent := v.storageMap[validateId]
	if !isPresent {
		return nil
	}

	return logs
}

var (
	validateSessionSingleton *ValidateIDToConsoleLogs
	validateSessionOnce      sync.Once
)

func (v *ValidateIDToConsoleLogs) deleteExpired() {
	v.mu.Lock()
	defer v.mu.Unlock()
	storageMapUpdated := v.storageMap
	for validateId, consoleLogs := range v.storageMap {
		// Since the response is not even populated yet, we cannot
		// delete it
		if !consoleLogs.GetIsReady() {
			continue
		}

		if consoleLogs.ShouldRemove() {
			// Response is expired and we can remove it.
			log.Debug(logTag, ": removing validate session details with ID: ", validateId)
			delete(storageMapUpdated, validateId)
		}
	}
}

func ValidateSessionOnce() *ValidateIDToConsoleLogs {
	validateSessionOnce.Do(func() {
		validateSessionSingleton = &ValidateIDToConsoleLogs{
			storageMap: make(map[string]*DelayedStageLogTracker, 0),
		}

		// Start the cronjob to delete expired entries
		expiredSessionRemovalJob := cron.New()
		expiredSessionRemovalJob.AddFunc("@every 1m", func() {
			log.Debug(logTag, ": running validate delete expired cronjob")
			validateSessionSingleton.deleteExpired()
		})
		expiredSessionRemovalJob.Start()
	})

	return validateSessionSingleton
}

// MergeStageLogs will merge all the logs for each stage
// and return an array of logs.
func MergeStageLogs(logsByStage map[string][]string) []string {
	allLogs := make([]string, 0)

	for _, stageLogs := range logsByStage {
		allLogs = append(allLogs, stageLogs...)
	}

	return allLogs
}
