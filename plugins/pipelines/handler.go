package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/ratelimiter"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/tracktime"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/openai"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/buger/jsonparser"
	"github.com/gdexlab/go-render/render"
	"github.com/gorilla/mux"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
)

// To return a map with all dependencies
// // For e.g if A -> B -> C then A depends on [B,C]
func getDependencyMap(stages map[string]StageStatus) map[string]map[string]bool {
	// handle nil map
	deepDependencyMap := make(map[string]map[string]bool)
	for stageId, stage := range stages {
		getDependencyMapHelper(stages, deepDependencyMap, stageId, stage)
	}
	return deepDependencyMap
}

func getDependencyMapHelper(
	stages map[string]StageStatus,
	deepDependencyMap map[string]map[string]bool,
	stageID string,
	stage StageStatus,
) map[string]map[string]bool {
	if len(stage.Needs) > 0 {
		for _, depId := range stage.Needs {
			if deepDependencyMap[stageID] == nil {
				deepDependencyMap[stageID] = map[string]bool{
					depId: true,
				}
			} else {
				deepDependencyMap[stageID][depId] = true
			}
			if depId != stageID { // avoids the infinite loop condition
				// call the recursive method
				getDependencyMapHelper(stages, deepDependencyMap, stageID, stages[depId])
			}
		}
	}
	return deepDependencyMap
}

func getStageID(stage ESPipelineStage) *string {
	id := stage.ID
	if id == nil {
		use := stage.Use.String()
		id = &use
	}
	return id
}

// getStageIndex will return the index of the stage in the
// passed pipeline
func getStageIndex(stageId string, pipeline ESPipelineDoc) int {
	for index, stage := range *pipeline.Stages {
		if (stage.ID != nil && stageId == *stage.ID) || (stage.Use != nil && stage.Use.String() == stageId) {
			return index
		}
	}

	return -1
}

// Returns the dependency map of stages
func getStageStatusMap(stages []ESPipelineStage) (map[string]StageStatus, error) {
	stageStatusMap := make(map[string]StageStatus)
	for _, stage := range stages {
		id := getStageID(stage)
		if id != nil {
			if stage.Needs != nil {
				stageStatusMap[*id] = StageStatus{
					IsCompleted: false,
					Needs:       *stage.Needs,
				}
			} else {
				stageStatusMap[*id] = StageStatus{
					IsCompleted: false,
				}
			}
		}
	}

	// stores the nth level of dependencies
	// For e.g if A -> B -> C then A depends on [B,C]
	dependencyMap := getDependencyMap(stageStatusMap)
	// Remove cyclic dependencies
	// For examples,
	// Stage A needs A must be ignored (direct dependency)
	// Stage A needs B and B needs A must be ignored (indirect dependency)
	for stageId, dependencies := range dependencyMap {
		if dependencies[stageId] {
			// cyclic dependency found
			return nil, errors.New(fmt.Sprint("cyclic dependency found for stage with ID: ", stageId))
		}
	}
	return stageStatusMap, nil
}

func getPipelineStageMiddleware(pipeline ESPipelineDoc, pipelineRoute ESPipelineRoutes) []middleware.Middleware {
	middlewares := []middleware.Middleware{}
	if pipeline.Stages != nil {
		for _, stage := range *pipeline.Stages {
			if stage.Enabled == nil || *stage.Enabled {
				if stage.Use != nil {
					// Pre-built stage
					switch *stage.Use {
					case Authorization:
						{
							shouldOmitOpValidation := false
							var cat string
							if reqCategory, ok := pipeline.Envs["category"]; ok {
								categoryAsString, ok := reqCategory.(string)
								if ok {
									cat = categoryAsString
								}
							}
							if pipelineRoute.Classify.Category != nil {
								cat = pipelineRoute.Classify.Category.String()
							}
							// Avoid operation validation for reactivesearch or analytics categories
							if cat == category.ReactiveSearch.String() ||
								cat == category.Analytics.String() {
								shouldOmitOpValidation = true
							}

							middlewares = append(middlewares, []middleware.Middleware{
								auth.BasicAuth(),
								ratelimiter.Limit(),
								validate.Sources(),
								validate.Referers(),
								validate.Indices(),
								validate.Category(),
								validate.PermissionExpiry(),
							}...)
							if !shouldOmitOpValidation {
								middlewares = append(middlewares, validate.Operation())
							}
						}

					}
				}
			}
		}
	}

	return middlewares
}

type StageStatus struct {
	IsRunning   bool // in progress status for stage; true when stage is started
	IsCompleted bool // completion status for stage
	IsSkipped   bool // skipped status for stage
	Needs       []string
}

type StageLogTracker struct {
	mu    sync.Mutex
	value map[string][]string
}

// To update the status for a stage by id
func (c *StageLogTracker) Put(stageId string, value *[]string) {
	c.mu.Lock()
	if value != nil {
		c.value[stageId] = *value
	}
	c.mu.Unlock()
}

// To update the status for a stage by id
func (c *StageLogTracker) Get(stageId string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	value := c.value[stageId]
	return value
}

type StageStatusMap struct {
	mu    sync.Mutex
	value map[string]StageStatus
}

// To retrive the stage status for a stage by id
func (c *StageStatusMap) Get(key string) StageStatus {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.value[key]
}

// To update the status for a stage by id
func (c *StageStatusMap) PutRunningStatus(key string, value bool) {
	c.mu.Lock()
	stageStatus := c.value[key]
	stageStatus.IsRunning = value
	c.value[key] = stageStatus
	c.mu.Unlock()
}

// PutSkippedStatus will update the skipped status of the stage
func (c *StageStatusMap) PutSkippedStatus(key string, value bool) {
	c.mu.Lock()
	stageStatus := c.value[key]
	stageStatus.IsSkipped = value
	c.value[key] = stageStatus
	c.mu.Unlock()
}

// To update the status for a stage by id
func (c *StageStatusMap) PutCompleteStatus(key string, value bool) {
	c.mu.Lock()
	stageStatus := c.value[key]
	stageStatus.IsCompleted = value
	c.value[key] = stageStatus
	c.mu.Unlock()
}

// To store request body to be cached
type CachedRequestContext struct {
	mu    sync.Mutex
	value []byte
}

// To retrive the request body to be cached
func (c *CachedRequestContext) Get() []byte {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.value
}

// To update the request body to be cached
func (c *CachedRequestContext) Put(value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = value
}

// To store reactive search query
type ReactiveSearchQueryContext struct {
	mu    sync.Mutex
	value *querytranslate.RSQuery
}

// To retrive the global script context
func (c *ReactiveSearchQueryContext) Get() *querytranslate.RSQuery {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.value
}

// To update the status for a stage by id
func (c *ReactiveSearchQueryContext) Put(value *querytranslate.RSQuery) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = value
}

var CONTEXT_RESERVED_KEYS = []string{"request", "response", "console_logs", "stageChanges"}

type GlobalScriptContext struct {
	mu    sync.Mutex
	value []byte
}

// To retrive the global script context
func (c *GlobalScriptContext) Get() []byte {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.value
}

// GetUpdatedContext gets the updated context based on the
// passed value and comparing it to the older context
func (c *GlobalScriptContext) GetUpdatedContext(value []byte, async bool, write bool) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var oldContextMap map[string]interface{}
	err := json.Unmarshal(c.value, &oldContextMap)
	if err != nil {
		log.Errorln(logTag, err)
		return nil, err
	}
	// convert value to map
	var newContextMap map[string]interface{}
	err2 := json.Unmarshal(value, &newContextMap)
	if err2 != nil {
		log.Errorln(logTag, err2)
		return nil, err2
	}
	// merge new context
	// new context properties would override the old context
	// async scripts won't be able to modify the core properties
	for k, v := range newContextMap {
		if async {
			if !util.Contains(CONTEXT_RESERVED_KEYS, k) {
				oldContextMap[k] = v
			}
		} else {
			oldContextMap[k] = v
		}
	}
	// convert to byte
	updatedContext, err := json.Marshal(oldContextMap)
	if err != nil {
		log.Errorln(logTag, err)
		return nil, err
	}
	if write {
		// write context
		c.value = updatedContext

	}
	return updatedContext, nil
}

// Put will update the global script context based on the passed
// value.
func (c *GlobalScriptContext) Put(value []byte, async bool) error {
	// Get the updated context
	_, err := c.GetUpdatedContext(value, async, true)
	if err != nil {
		return err
	}
	return nil
}

// To update the status for a stage by id
func (c *GlobalScriptContext) GetMap() map[string]interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	var currentContextMap = make(map[string]interface{})
	json.Unmarshal(c.value, &currentContextMap)
	return currentContextMap
}

func (pipeline ESPipelineDoc) pipelineHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !util.ValidatePlans(validPlans, util.GetFeaturePipelines()) {
			telemetry.WriteBackErrorWithTelemetry(r, w, "Route is invoked by pipeline id: "+*pipeline.ID+". Pipeline feature is not available for your plan.", http.StatusPaymentRequired)
			return
		}
		start := time.Now()

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		// Prepare script context
		// extract request headers
		requestHeaders := make(map[string]string)
		for k := range r.Header {
			requestHeaders[k] = r.Header.Get(k)
		}

		// Check if `debug` flag is passed
		debugValue := r.URL.Query()["debug"]
		isDebug := false
		if len(debugValue) != 0 && debugValue[0] == "true" {
			isDebug = true
		}

		// Capture the pipeline details and request body and store
		// it to the context
		pipelineLog, pipelineFetchErr := FromContext(r.Context())
		shouldUpdateLog := pipelineFetchErr == nil

		if shouldUpdateLog {
			*pipelineLog.PipelineID = *pipeline.ID

			*pipelineLog.Request = Request{
				Body:    string(reqBody),
				Method:  r.Method,
				Headers: r.Header,
				URI:     r.URL.Path,
			}

			// NOTE: No need to update the value in the request context
			// since we are updating the value and not the address so
			// changes will be picked up automatically.
		}

		passedEnvs := pipeline.getPipelineEnvironments(r, reqBody)

		// Check if method is passed, if not passed, set it to the
		// method that the pipeline is getting invoked by.
		_, ok := passedEnvs["method"].(string)
		if !ok {
			passedEnvs["method"] = r.Method
		}

		pipelineExecutionContext := PipelineExecutionContext{
			envs: passedEnvs,
			request: PipelineExecutionRequest{
				Body:    reqBody,
				Headers: requestHeaders,
			},
		}
		scriptContext, pipelineErr := pipeline.executePipeline(pipelineExecutionContext, r.Context(), true, pipelineLog, false, true, isDebug, r)

		// Capture the pipeline time took
		pipelineTook := int(time.Since(start).Milliseconds())
		if shouldUpdateLog {
			*pipelineLog.Took = pipelineTook
		}

		if pipelineErr != nil {
			code := http.StatusInternalServerError
			if pipelineErr.Code != 0 {
				code = pipelineErr.Code
			}
			telemetry.WriteBackErrorWithTelemetry(r, w, pipelineErr.Err.Error(), code)
			return
		}
		statusCode := http.StatusOK
		body := ""
		responseMap, ok := scriptContext["response"].(map[string]interface{})
		if ok {
			code, ok := responseMap["code"].(float64)
			if ok {
				if code != 0 {
					statusCode = int(code)
				}
			}
			// apply response headers
			headers, ok := responseMap["headers"].(map[string]interface{})
			if ok {
				for k, v := range headers {
					headerValue, ok := v.(string)
					if ok {
						w.Header().Set(k, headerValue)
					}
				}
			}
			// apply response body
			responseBody, ok := responseMap["body"].(string)

			if ok {
				// Check if settings.took is present, if it is present
				// update it with the pipelineTook value
				//
				// If it doesn't exist or error is thrown, skip adding
				// the took key
				_, valueType, _, err := jsonparser.Get([]byte(responseBody), "settings", "took")
				tookAsBytes, tookErr := json.Marshal(pipelineTook)

				if valueType != jsonparser.NotExist && err == nil && tookErr == nil {
					// Marshal the took into bytes
					bodyWithTook, err := jsonparser.Set([]byte(responseBody), tookAsBytes, "settings", "took")
					if err == nil {
						responseBody = string(bodyWithTook)
					}
				}

				body = responseBody
			}

			// Parse the debug values if it is present
			if isDebug {
				// We will have to remove the `took` key from the finalScriptContext
				delete(scriptContext, "took")

				// Extract the `stageChanges` from the script context and inject in the
				// settings object inside the `debug` key.
				stageChangesInjected, stageChangesPresent := scriptContext["stageChanges"]
				if !stageChangesPresent {
					// This is a very unlikely thing to happen, but we should throw
					// an error in this case.
					errMsg := fmt.Sprint("error while parsing stageChanges to inject in `settings.debug` key, not injected into context!")
					log.Errorln(logTag, ": ", errMsg)
					telemetry.WriteBackErrorWithTelemetry(r, w, errMsg, http.StatusInternalServerError)
					return
				}

				// Inject the stageChanges inside settings now.
				stageChangesMarshalled, marshalErr := json.Marshal(stageChangesInjected)
				if marshalErr != nil {
					errMsg := fmt.Sprint("error while marshalling stage changes to inject in `debug` key: ", marshalErr.Error())
					log.Errorln(logTag, ": ", errMsg)
					telemetry.WriteBackErrorWithTelemetry(r, w, errMsg, http.StatusInternalServerError)
					return
				}
				bodyWithDebug, debugInjectErr := jsonparser.Set([]byte(body), stageChangesMarshalled, "settings", "debug")
				if debugInjectErr != nil {
					errMsg := fmt.Sprint("error while injecting the `debug` key in settings: ", debugInjectErr.Error())
					log.Errorln(logTag, ": ", errMsg)
					telemetry.WriteBackErrorWithTelemetry(r, w, errMsg, http.StatusInternalServerError)
					return
				}

				body = string(bodyWithDebug)
			}
		}

		// Insert the X-Took value
		w.Header().Set("X-Took", fmt.Sprint(pipelineTook))
		util.WriteBackRaw(w, []byte(body), statusCode)
	}
}

// Retruns the environments for pipeline
func (pipeline ESPipelineDoc) getPipelineEnvironments(r *http.Request, reqBody []byte) map[string]interface{} {
	// Prepare envs

	// Declare default empty strings for passed category
	// and ACL
	var passedCategory = ""
	var passedACL = ""

	// Check if it is a validate request
	reqPath := r.URL.Path
	isValidate := reqPath == "/_pipeline/validate"

	// Extract the request category
	reqCategory, err := category.FromContext(r.Context())
	if err != nil {
		log.Errorln(logTag, ":", "Couldn't extract category from ctx")
	}

	// Extract the category from the request context
	//
	// Extract the category only if it's not a validate
	// endpoint, else we need to skip it.
	//
	// This check is put in place because in validate requests
	// the category will always be `pipelines` and the user
	// passed category might be overwritten.
	if !isValidate {
		passedCategory = reqCategory.String()
	}

	reqAcl, err := acl.FromContext(r.Context())
	if err == nil {
		passedACL = reqAcl.String()
	}

	ip := iplookup.FromRequest(r)

	var clientIPv4 string

	var clientIPv6 string

	ipv4 := rules.GetClientIP4(ip)
	if ipv4 != "" {
		clientIPv4 = ipv4
	} else {
		ipv6 := rules.GetClientIP6(ip)
		if ipv6 != "" {
			clientIPv6 = ipv6
		}
	}

	indices, err := index.FromContext(r.Context())
	if err != nil {
		log.Errorln(logTag, ":", err)
	}

	urlValues := map[string]string{}
	for k := range r.URL.Query() {
		urlValues[k] = r.URL.Query().Get(k)
	}
	requestEnvironments := rules.TriggerEnvironmentsToEvaluate{
		Index:        indices,
		Origin:       r.Host,         // https://my-search.domain.com
		Referer:      r.Referer(),    // https://my-search.domain.com/path?q=hello
		Path:         r.URL.Path,     // /_doc/test
		URLValues:    urlValues,      // {'q': 'hello'}
		Category:     passedCategory, // docs
		ACL:          passedACL,      // bulk
		IPv4:         clientIPv4,     // 29.120.12.12
		IPv6:         clientIPv6,     // 2001:db8:3333:4444:5555:6666:7777:8888
		OpenAIConfig: openai.Instance().GetConfig(),
	}

	// Unmarshal the passed body in RSQuery to use it for extracting
	// things like category and query.
	var rsAPIBody querytranslate.RSQuery
	unmarshalErr := json.Unmarshal(reqBody, &rsAPIBody)

	if unmarshalErr != nil {
		log.Warnln(logTag, ":", unmarshalErr.Error())
	}

	// NOTE: There is no need to check if err is nil but it's just a failsafe to
	// make sure no segmentation errors happen.
	if reqCategory != nil && *reqCategory == category.ReactiveSearch && unmarshalErr == nil {
		// extract Envs from request body
		environments := querytranslate.ExtractEnvsFromRequest(rsAPIBody)
		if environments.Query != nil {
			requestEnvironments.Query = *environments.Query
		}
		requestEnvironments.Filter = rules.ParseFilters(environments.TermFilters)
	}

	requestEnvironmentsMap := rules.TriggerEnvsToMap(requestEnvironments)
	scriptEnvs := requestEnvironmentsMap
	// Apply pipeline envs
	// pipeline envs has the highest priority
	for k, v := range pipeline.Envs {
		scriptEnvs[k] = v
	}

	// Parse whether the request is TLS or not
	scriptEnvs["isTLS"] = r.TLS != nil

	// Extract the query if it is present.
	if len(rsAPIBody.Query) > 0 {
		var valueToUse *interface{}

		// Use the first query that has the `value` field present.
		for _, queryEach := range rsAPIBody.Query {
			if queryEach.Value != nil {
				valueToUse = queryEach.Value
			}
		}

		if valueToUse != nil {
			// Set the value in the scriptEnvs
			scriptEnvs["query"] = valueToUse
		}
	}

	return scriptEnvs
}

type PipelineExecutionRequest struct {
	Body    []byte
	Headers map[string]string
}

type PipelineExecutionContext struct {
	envs    map[string]interface{}
	request PipelineExecutionRequest
}

// list of pre-built stages that can be executed asynchronously
var asyncPrebuiltStages = []string{ElasticSearchQuery.String(), HttpRequest.String(), MongoDBQuery.String(), Boost.String(), OpenAIEmbeddings.String(), OpenAIEmbeddingsIndex.String()}

func executeStage(
	dependencyStage ESPipelineStage,
	stageStatusMap *StageStatusMap,
	stageLogTracker *StageLogTracker,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	cachedRequestContext *CachedRequestContext,
	stageErrorTracker chan Error,
	executionCtx PipelineExecutionContext,
	stageChanges *[]*StageChange,
	stagesInvoked *PipelineInvokeMap,
	originalPipeline ESPipelineDoc,
	req *http.Request,
	contextDiffWg *sync.WaitGroup,
	bgScriptWg *sync.WaitGroup,
	bgStagesToTime *BackgroundStageToTime,
	runInBg bool,
) (bool, *Error) {
	stageStart := time.Now()

	stageId := getStageID(dependencyStage)

	var globalCtx rules.ScriptContext
	unmarshalErr := json.Unmarshal(globalScriptContext.Get(), &globalCtx)
	if unmarshalErr != nil {
		return true, &Error{
			Err:  appendStageIdToErr(fmt.Errorf("Error while unmarshalling script envs: %s", unmarshalErr.Error()), *stageId),
			Code: http.StatusInternalServerError,
		}
	}

	scriptEnvs := globalCtx.Environments

	if dependencyStage.Enabled != nil && !*dependencyStage.Enabled {
		return false, nil
	}

	if stageStatusMap.Get(*stageId).IsCompleted ||
		stageStatusMap.Get(*stageId).IsRunning || stageStatusMap.Get(*stageId).IsSkipped {
		return false, nil
	}

	// Check trigger and accordingly skip if required.
	shouldStageRun, triggerCheckErr := runStageTrigger(originalPipeline, executionCtx, *stageId, globalScriptContext.Get())
	if triggerCheckErr != nil {
		// TODO: Update response returned to the user
		return true, &Error{
			Err:  appendStageIdToErr(triggerCheckErr, *stageId),
			Code: http.StatusBadRequest,
		}
	}

	if !shouldStageRun {
		log.Warnln(logTag, fmt.Sprintf(": skipping execution of stage with ID: %s because trigger returned false", *stageId))
		stageStatusMap.PutSkippedStatus(*stageId, true)
		return false, nil
	}

	log.Debugln(logTag, " EXECUTING =============", *stageId)
	hasBoostStage := false
	for _, v := range *originalPipeline.Stages {
		isEnabled := v.Enabled == nil || *v.Enabled
		if v.Use != nil && *v.Use == Boost && isEnabled {
			hasBoostStage = true
		}
	}

	// Capture the stage before it gets executed.
	//
	// Set default values for error and took, will be updated
	// in the following code.
	var isStageError bool = false
	var stageTimeTook int = 0
	var isStageExecuted bool = false

	// Make sure to lock before writing the stages invoked
	stagesInvoked.AddStage(*stageId, &isStageError, &stageTimeTook, &isStageExecuted)

	isPrebuiltStage := dependencyStage.Use != nil

	if (dependencyStage.Async != nil && *dependencyStage.Async) &&
		(!isPrebuiltStage ||
			util.Contains(asyncPrebuiltStages, (*dependencyStage.Use).String())) {
		// Update Stage status as running
		stageStatusMap.PutRunningStatus(*stageId, true)

		// Set stage executed as true
		isStageExecuted = true

		log.Debug(logTag, ":starting exec in go routine")
		// execute stage in go routine
		go func(stageDetails ESPipelineStage) {
			var updatedScriptContext []byte
			var err *Error
			id := getStageID(stageDetails)
			// Update stage status map
			defer stageStatusMap.PutRunningStatus(*id, false)
			defer stageStatusMap.PutCompleteStatus(*id, true)
			defer log.Debug(logTag, ": exiting go routine for: ", *id)

			if dependencyStage.Use != nil {
				// pre-built stage
				updatedScriptContext, _, err = executePreBuiltStage(
					stageDetails,
					globalScriptContext,
					rsAPIRequest,
					cachedRequestContext,
					scriptEnvs,
					true,
					&stageStart,
					req,
					hasBoostStage,
				)
				if err != nil {
					err = &Error{
						Err: appendStageIdToErr(err.Err, *stageId),
					}
				}
			} else {
				var logs *[]string
				var err2 error

				timeout := 10 * time.Second
				if runInBg {
					timeout = 30 * time.Second

					// Add wg to make sure the bg scripts are captured properly
					bgScriptWg.Add(1)
					defer func() {
						log.Debug(logTag, ": Removing entry from wg")
						bgScriptWg.Done()
					}()
				}

				log.Debug(logTag, ": timeout for stage: ", *id, timeout)

				// execute script in go routine
				log.Debug(logTag, ": running script")
				updatedScriptContext, logs, err2 = rules.RunScript(globalScriptContext.Get(), *stageDetails.Script, timeout)
				log.Debug(logTag, ": script done!")

				// write logs
				stageLogTracker.Put(*stageId, logs)
				if err2 != nil {
					err = &Error{
						Err: appendStageIdToErr(err2, *stageId),
					}
				}
			}

			// Capture the stage change
			//
			// In async stages, it is possible that the stage
			// with ID is already created.
			// We will check if the stage is already created,
			// if so then return this stage else just create a
			// new one and append it to the array
			stageChange := GetStageChange(*stageId, stageChanges)
			stageChange.Took = &stageTimeTook
			logsInstance := logs.Instance()

			if runInBg {
				(*bgStagesToTime).Add(*id, &stageTimeTook)
			}

			if updatedScriptContext != nil {
				// Capture stage context diff
				updatedContext, err := globalScriptContext.GetUpdatedContext(updatedScriptContext, true, false)
				if err == nil {
					// Run the context diff generation in the background to save time
					// in pipeline execution.
					if logsInstance.IsDiffingDisabled() {
						updateCtxAsStr := string(updatedContext)
						stageChange.Context = &updateCtxAsStr
					} else {
						// Add a waitgroup to make sure the diffs are waited for
						contextDiffWg.Add(1)

						globalContextCopy := globalScriptContext.Get()
						go func(runInBg bool) {
							contextDiffDelta := getStageContextDiff(globalContextCopy, updatedContext)
							stageChange.Context = &contextDiffDelta
							log.Debug(logTag, ": stage with ID: ", stageChange.ID, ": done")
							defer contextDiffWg.Done()
						}(runInBg)
					}
				}
			}

			if err != nil {
				log.Errorln(logTag, ":", err.Err.Error())

				stageTimeTook = int(time.Since(stageStart).Milliseconds())
				isStageError = true
				// Capture stage error
				errString := err.Err.Error()
				stageChange.Error = &errString

				err.Err = appendStageIdToErr(err.Err, *stageId)

				// if `ContinueOnError` is set to `false` then return the error
				if stageDetails.ContinueOnError != nil && !*stageDetails.ContinueOnError {
					// Report error
					stageErrorTracker <- *err
					return
				}
			} else {
				// for async go stages avoid updating the pre-defined keys
				globalScriptContext.Put(updatedScriptContext, true)
			}

			stageTimeTook = int(time.Since(stageStart).Milliseconds())
		}(dependencyStage)
	} else {
		var updatedScriptContext []byte
		var err *Error
		var shouldStopExecution bool
		// Update stage status map
		defer stageStatusMap.PutRunningStatus(*stageId, false)
		defer stageStatusMap.PutCompleteStatus(*stageId, true)

		isStageExecuted = true

		if dependencyStage.Use != nil {
			// pre-built stage
			updatedScriptContext, shouldStopExecution, err = executePreBuiltStage(
				dependencyStage,
				globalScriptContext,
				rsAPIRequest,
				cachedRequestContext,
				scriptEnvs,
				false,
				&stageStart,
				req,
				hasBoostStage,
			)
		} else {
			var logs *[]string
			var err2 error
			// execute script in go routine
			updatedScriptContext, logs, err2 = rules.RunScript(globalScriptContext.Get(), *dependencyStage.Script, 10*time.Second)
			// write logs
			stageLogTracker.Put(*stageId, logs)
			if err2 != nil {
				err = &Error{
					Err: err2,
				}
			}
		}

		stageChange := GetStageChange(*stageId, stageChanges)
		stageChange.Took = &stageTimeTook
		logsInstance := logs.Instance()

		if updatedScriptContext != nil {
			// Capture stage context diff
			updatedContext, err := globalScriptContext.GetUpdatedContext(updatedScriptContext, false, false)
			if err == nil {
				// Run the context diff generation in the background to save time
				// in pipeline execution.
				if logsInstance.IsDiffingDisabled() {
					updateCtxAsStr := string(updatedContext)
					stageChange.Context = &updateCtxAsStr
				} else {
					// Add a waitgroup to make sure the diffs are waited for
					contextDiffWg.Add(1)

					globalContextCopy := globalScriptContext.Get()
					go func() {
						contextDiffDelta := getStageContextDiff(globalContextCopy, updatedContext)
						stageChange.Context = &contextDiffDelta
						log.Debug(logTag, ": stage with ID: ", stageChange.ID, ": done")
						defer contextDiffWg.Done()
					}()
				}
			}
		}

		if err != nil {
			log.Errorln(logTag, ":", err.Err.Error())

			stageTimeTook = int(time.Since(stageStart).Milliseconds())
			isStageError = true
			// Capture stage error
			errString := err.Err.Error()
			stageChange.Error = &errString

			err.Err = appendStageIdToErr(err.Err, *stageId)

			// if `ContinueOnError` is set to `false` then return the error
			if dependencyStage.ContinueOnError != nil && !*dependencyStage.ContinueOnError {
				return shouldStopExecution, err
			}
		} else {
			// update script context
			globalScriptContext.Put(updatedScriptContext, false)
		}

		stageTimeTook = int(time.Since(stageStart).Milliseconds())

		// if `ContinueOnError` is set to `false`
		if dependencyStage.ContinueOnError != nil && !*dependencyStage.ContinueOnError {
			// check for error response
			var scriptContext rules.ScriptContext
			err := json.Unmarshal(updatedScriptContext, &scriptContext)
			if err == nil {
				// ignore marshal error
				// check if response code is >= 4xx
				if scriptContext.Response.Code >= 400 {
					// stop execution
					return true, nil
				}
			}
		}
		return shouldStopExecution, nil
	}
	stageTimeTook = int(time.Since(stageStart).Milliseconds())
	return false, nil
}

type ExecutePipelineResponse struct {
	Request      rules.ScriptRequest    `json:"request"`
	Response     rules.ScriptResponse   `json:"response"`
	Environments map[string]interface{} `json:"envs"`
	Logs         map[string][]string    `json:"console_logs"`
}

func (pipeline ESPipelineDoc) executePipeline(pipelineExecutionContext PipelineExecutionContext, reqContext context.Context, captureAnalytics bool, pipelineLog *PipelineLog, isValidate bool, parseDiffs bool, isDebug bool, req *http.Request) (map[string]interface{}, *Error) {
	// Start timer to record script execution time.
	start := time.Now()

	// Create a container for stages invoked by the pipeline
	//
	// This container will be passed to executeStage and that function
	// will write to it if the stage is actually executed and pickup
	// time took and errors accordingly.
	stagesInvokedMap := PipelineInvokeMap{value: make(map[string]PipelineInvokeStage)}

	var timeTook int = int(time.Since(start).Milliseconds())

	// Extract the waitgroup from context that was injected by the logs recorder
	// to wait for the log diff generations to complete.
	//
	// If context is passed as nil, init with a new context. The context will
	// be empty only for test cases that call this method.
	if reqContext == nil {
		wgArr := []*sync.WaitGroup{new(sync.WaitGroup), new(sync.WaitGroup), new(sync.WaitGroup)}
		wgCtx := WgNewContext(context.Background(), &wgArr)
		reqContext = wgCtx
	}

	logUpdateWgArr, wgErr := WgFromContext(reqContext)
	if wgErr != nil {
		errMsg := fmt.Sprint("error while getting wg for log update: ", wgErr)
		log.Errorln(logTag, ": ", errMsg)

		// Thrown an error if not isValidate, else initialize
		if !isValidate {
			return make(map[string]interface{}), &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		} else {
			newWgArr := []*sync.WaitGroup{new(sync.WaitGroup), new(sync.WaitGroup), new(sync.WaitGroup)}
			logUpdateWgArr = &newWgArr
		}

	}

	// Create a wait group to wait for all the background scripts to complete
	// before capturing the logs and pipeline invocation.
	var bgScriptWg = (*logUpdateWgArr)[2]

	// If the request is a validate or a debug call, we need to generate and ID
	// so that if we have any background scripts, the logs for those scripts
	// will be returned later on.
	var validateId = ""
	shouldGenerateValidateId := isValidate || isDebug

	// Call the create pipeline record to record the pipeline invocation
	// details.
	//
	// Since we're using defer, the function will be executed once everything
	// is completed, i:e all stages are executed so we need to pass the
	// stages and took as a pointer instead of by value because the values
	// might change after the following line and we want the changes to be
	// reflected.
	//
	// Conditionally call this if capturing analytics is set to true
	if captureAnalytics {
		defer func(bgScriptWg *sync.WaitGroup) {
			go func(bgScriptWg *sync.WaitGroup) {
				// Fetch the start time from the time tracker
				startAsPtr, fetchErr := tracktime.FromTimeTrackerContext(req.Context())
				if fetchErr != nil {
					log.Warnln(logTag, ": error while fetching time from ctx to calculate time taken: ", fetchErr.Error())
					return
				}
				start := *startAsPtr
				// capture time took after pipeline execution but before bg scripts
				timeTook = int(time.Since(start).Milliseconds())
				// Wait for bg scripts to complete before capturing the invocation
				// record.
				log.Debug(logTag, ": waiting for bg scripts to complete (if any)")
				bgScriptWg.Wait()
				log.Debug(logTag, ": done waiting! Creating invocation record.")
				if inv := Instance().invocationEs; inv != nil {
					inv.createPipelineInvokeRecord(*pipeline.ID, *pipeline.LiveVersion, stagesInvokedMap.GetStagesMap(), timeTook)
				}
			}(bgScriptWg)
		}(bgScriptWg)
	}

	// Determine if log should be updated
	shouldUpdateLog := pipelineLog != nil

	var stageChanges = make([]*StageChange, 0)

	// tracks the logs for stages by stage id
	stageLogTracker := StageLogTracker{
		value: make(map[string][]string),
	}

	// Track the time of the stages run in the background
	bgStagesToTime := BackgroundStageToTime{
		storage: make(map[string]*int, 0),
	}

	// Create a wait group to wait for all the stage context diffs to complete.
	//
	// This step is necessary to make sure that the log diffs are generated properly
	// before the logs are written.
	var stageDiffWg = (*logUpdateWgArr)[1]

	defer func(bgScriptWg *sync.WaitGroup, pipelineLog *PipelineLog, stageChanges *[]*StageChange, shouldUpdateLog *bool, contextDiffWg *sync.WaitGroup, logWg *sync.WaitGroup, validateId *string, stageLogTracker *StageLogTracker, bgStagesToTime *BackgroundStageToTime) {
		go func(bgScriptWg *sync.WaitGroup, pipelineLog *PipelineLog, stageChanges *[]*StageChange, shouldUpdateLog *bool, contextDiffWg *sync.WaitGroup, logWg *sync.WaitGroup) {
			// Wait for the logs to update
			logWg.Add(1)

			// Wait for bg scripts to complete before capturing the invocation
			// record.
			log.Debug(logTag, ": waiting for bg scripts to complete (if any)")
			bgScriptWg.Wait()
			log.Debug(logTag, ": done waiting! Updating the logs.")

			log.Debug(logTag, " Stage changes len: ", len(*stageChanges))

			startUpdateLogs(pipelineLog, stageChanges, *shouldUpdateLog, stageDiffWg, logWg)
		}(bgScriptWg, pipelineLog, stageChanges, shouldUpdateLog, stageDiffWg, logWg)

		// Update the console logs against the validate ID so that the updated console
		// logs can be fetched.
		go func(validateId *string, stageLogTracker *StageLogTracker, bgStagesToTime *BackgroundStageToTime) {
			// Wait for the bg scripts to complete
			log.Debug(logTag, ": waiting for bg scripts to complete (if any)")
			bgScriptWg.Wait()
			log.Debug(logTag, ": done waiting! Updating the console logs.")

			// Add logic to update the validateId with the logs
			ValidateSessionOnce().AddLogs(*validateId, stageLogTracker, bgStagesToTime)

			log.Debug(logTag, ": Logs are: ", render.AsCode(stageLogTracker.value))
			log.Debug(logTag, ": background stages time: ", render.AsCode(bgStagesToTime))
		}(validateId, stageLogTracker, bgStagesToTime)
	}(bgScriptWg, pipelineLog, &stageChanges, &shouldUpdateLog, stageDiffWg, (*logUpdateWgArr)[0], &validateId, &stageLogTracker, &bgStagesToTime)

	// Inject the global vars to envs so that it can be resolved
	// directly.
	envsPassed := pipelineExecutionContext.envs
	InjectEnvs(&envsPassed)
	pipelineExecutionContext.envs = envsPassed

	requestHeaders := pipelineExecutionContext.request.Headers
	if requestHeaders == nil {
		requestHeaders = make(map[string]string)
	}

	responseHeaders := make(map[string]string)
	if pipeline.ID != nil {
		responseHeaders[XPipelineID] = *pipeline.ID
	}

	// Generate the request URL
	scheme := "http"
	if req != nil && req.TLS != nil {
		scheme += "s"
	}

	URL := ""
	// use any user-defined search URL env, if present
	searchURL, ok := pipelineExecutionContext.envs["searchURL"].(string)
	if ok {
		URL = searchURL
	} else {
		searchURL, ok := pipelineExecutionContext.envs["url"].(string)
		if ok {
			URL = searchURL
		} else {
			searchURL, ok := pipelineExecutionContext.envs["URL"].(string)
			if ok {
				URL = searchURL
			}
		}
	}

	requestURL := ""
	requestMethod := ""
	if req != nil {
		requestURL = fmt.Sprintf("%s://%s%s", scheme, req.Host, req.URL.RequestURI())
		requestMethod = req.Method
	}
	if URL != "" {
		requestURL = URL
		// TODO: Remove this log line
		fmt.Println("updating fallback URL to one defined in env: ", requestURL)
	}

	scriptContext := ExecutePipelineResponse{
		Request: rules.ScriptRequest{
			Body:    string(pipelineExecutionContext.request.Body),
			Headers: requestHeaders,
			URL:     requestURL,
			Method:  requestMethod,
		},
		// Default response to write.
		// It is the responsibility of stage to write the response if
		// ElasticsearchQuery stage is not present
		Response: rules.ScriptResponse{
			Code:    http.StatusOK,
			Body:    "",
			Headers: responseHeaders,
		},
		Environments: pipelineExecutionContext.envs,
		Logs:         make(map[string][]string),
	}
	// tracks the error status for async stages
	stageErrorTracker := make(chan Error)

	scriptContextBytes, err := json.Marshal(scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", "Couldn't extract category from ctx")
		return make(map[string]interface{}), &Error{
			Err: err,
		}
	}

	if shouldUpdateLog {
		stringedContext := string(scriptContextBytes)
		pipelineLog.Context = &stringedContext
	}

	globalScriptContext := GlobalScriptContext{
		value: scriptContextBytes,
	}

	if pipeline.Stages != nil {
		// Perform followings changes in stages if `boost` stage exists
		// 1. Set `Async` as `true` for all boost stages
		// 2. Add boost stages (ids) to the `needs` property of `elasticsearchQuery` stage
		// 3. Add the reactivesearchQuery as a `needs` for all the boost stages
		boostStages := []string{}
		boostIndices := []int{}

		// Find a list of all the stages that are not required
		// by any other stage.
		//
		// We can keep a list of all stages and remove any stage
		// that appears as a dependency in any other stage.
		stagesNeededByOthers := map[string]bool{}

		reactivesearchId := ""
		for i, p := range *pipeline.Stages {
			if p.Use != nil {
				isEnabled := p.Enabled == nil || *p.Enabled
				if *p.Use == Boost && isEnabled {
					async := true
					// Set async as `true` for boost stages
					(*pipeline.Stages)[i].Async = &async
					id := getStageID(p)
					if id != nil {
						boostStages = append(boostStages, *id)
						boostIndices = append(boostIndices, i)
					}
				}
				if *p.Use == ElasticSearchQuery && isEnabled {
					transformToRSAPIResponse := false
					if reqCategory, ok := scriptContext.Environments["category"]; ok {
						categoryAsString, ok := reqCategory.(string)
						if ok {
							if categoryAsString == category.ReactiveSearch.String() {
								transformToRSAPIResponse = true
							}
						}
					}
					URL := ""
					// use any user-defined search URL env, if present
					searchURL, ok := scriptContext.Environments["searchURL"].(string)
					if ok {
						URL = searchURL
					} else {
						searchURL, ok := scriptContext.Environments["url"].(string)
						if ok {
							URL = searchURL
						} else {
							searchURL, ok := scriptContext.Environments["URL"].(string)
							if ok {
								URL = searchURL
							}
						}
					}
					inputVariables := make(map[string]interface{})
					parsedInputs, err := getInputValuesFromContext(p.Inputs, inputVariables)
					if err == nil {
						inputs, err := getInputs(ElasticsearchQueryInput{
							URL:                           &URL,
							ParseResponseToReactivesearch: &transformToRSAPIResponse,
						}, parsedInputs)
						if err == nil && inputs.ParseResponseToReactivesearch != nil {
							transformToRSAPIResponse = *inputs.ParseResponseToReactivesearch
						}
					}
					// Update `needs` property of elasticsearchQuery
					if transformToRSAPIResponse {
						if p.Needs == nil {
							(*pipeline.Stages)[i].Needs = &boostStages
						} else {
							needs := append(*p.Needs, boostStages...)
							(*pipeline.Stages)[i].Needs = &needs
						}
					}
				}

				if *p.Use == ReactiveSearchQuery && isEnabled {
					reactivesearchStageId := getStageID(p)
					if reactivesearchStageId == nil {
						// Assign a new ID
						rsId := "rs__stage__id"
						(*pipeline.Stages)[i].ID = &rsId
						reactivesearchStageId = &rsId
					}
					reactivesearchId = *reactivesearchStageId
					log.Debug(logTag, ": rs stage ID for boost needs: ", reactivesearchId)
				}
			}

			if p.Needs == nil || len(*p.Needs) == 0 {
				continue
			}

			// Seems like we have a list of stages that are required
			// by this stage.
			for _, neededStage := range *p.Needs {
				stagesNeededByOthers[neededStage] = true
			}
		}

		// Inject the needs property for all boost stages.
		boostNeeds := []string{
			reactivesearchId,
		}
		for _, boostIndex := range boostIndices {
			(*pipeline.Stages)[boostIndex].Needs = &boostNeeds
		}

		// Execute pipeline stages
		// 1. Build dependency map based on the `needs` property.
		//    Dependency map would track the current status (completed) of stages.
		//    It would throw error if cyclic dependency present.

		stagesStatusMap, err := getStageStatusMap(*pipeline.Stages)
		if err != nil {
			// Capture the time took
			timeTook = int(time.Since(start).Milliseconds())

			finalScriptContext := globalScriptContext.GetMap()

			// Inject logs
			finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

			// Inject validate
			if isValidate {
				finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
			}

			log.Errorln(logTag, ": ", err.Error())
			return finalScriptContext, &Error{
				Err: err,
			}
		}
		stageStatusMap := StageStatusMap{
			value: stagesStatusMap,
		}
		// TODO: Set stage status for pre-built stages to completed
		// stores the nth level of dependencies
		// For e.g if A -> B -> C then A depends on [B,C]
		dependencyMap := getDependencyMap(stageStatusMap.value)

		// marshal to RS API request body
		rsAPIContext := ReactiveSearchQueryContext{}

		// context to stored request body to be cached
		cachedRequestContext := CachedRequestContext{}

		shouldSkipStage := false
		shouldContinueInBg := false

		for _, mainStage := range *pipeline.Stages {
			mainStageId := getStageID(mainStage)

			// Set shouldSkipStage to `false` for every iteration
			shouldSkipStage = false

			// Set shouldContinueInBg to `false` for every iteration
			isStageNeededByOther := stagesNeededByOthers[*mainStageId]
			shouldContinueInBg = !isStageNeededByOther && (mainStage.Async != nil && *mainStage.Async)
			log.Debug(logTag, ": should continue stage in bg `", *mainStageId, "` is: ", shouldContinueInBg)

			if shouldContinueInBg && validateId == "" && shouldGenerateValidateId {
				validateId = generateValidateId()
			}

			if mainStageId != nil {
				// check for error in go routines
				select {
				case stageError := <-stageErrorTracker:
					scriptContext.Logs = stageLogTracker.value

					// Capture stage took and set error status
					timeTook = int(time.Since(start).Milliseconds())

					finalScriptContext := globalScriptContext.GetMap()

					// Inject logs
					finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

					// Inject validate
					if isValidate {
						finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
					}

					return finalScriptContext, &stageError
				default:
				}
				// Avoid running the completed/running script
				if stageStatusMap.Get(*mainStageId).IsCompleted ||
					stageStatusMap.Get(*mainStageId).IsRunning {
					continue
				}
				// if stage is not completed
				// validate trigger to see if the stage should be executed
				// check for the status of dependencies
				// if any of the dependency is in pending state
				// then first resolve the dependencies then execute the script

				// TODO: kill all the go routines after the stages are done
				// There could be a case where an async stage doesn't have a dependent and modify the response

				// timeout error
				// Timeout is 10s for all stages
				// Timeout is 10s for a script
				if dependencies, ok := dependencyMap[*mainStageId]; ok {
					// wait for dependencies to complete
					for start := time.Now(); ; {
						completedStages := 0
						// check for error in go routines
						select {
						case stageError := <-stageErrorTracker:
							scriptContext.Logs = stageLogTracker.value

							// Capture stage took and set error status
							timeTook = int(time.Since(start).Milliseconds())

							finalScriptContext := globalScriptContext.GetMap()

							// Inject logs
							finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

							// Inject validate
							if isValidate {
								finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
							}

							return finalScriptContext, &stageError
						default:
						}
						for stageId := range dependencies {
							if !stageStatusMap.Get(stageId).IsCompleted && !stageStatusMap.Get(stageId).IsSkipped {
								for _, dependencyStage := range *pipeline.Stages {
									// search of the stage details
									dependencyStageId := getStageID(dependencyStage)
									if dependencyStageId != nil && *dependencyStageId == stageId {
										shouldStopExecution, err := executeStage(dependencyStage,
											&stageStatusMap,
											&stageLogTracker,
											&globalScriptContext,
											&rsAPIContext,
											&cachedRequestContext,
											stageErrorTracker,
											pipelineExecutionContext,
											&stageChanges,
											&stagesInvokedMap,
											pipeline,
											req,
											stageDiffWg,
											bgScriptWg,
											&bgStagesToTime,
											false,
										)
										if err != nil {
											scriptContext.Logs = stageLogTracker.value

											// Capture stage took and set error status
											timeTook = int(time.Since(start).Milliseconds())

											finalScriptContext := globalScriptContext.GetMap()

											// Inject logs
											finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

											// Inject validate
											if isValidate {
												finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
											}

											return finalScriptContext, err
										}
										if shouldStopExecution {
											finalScriptContext := globalScriptContext.GetMap()

											// Inject logs
											finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

											// Inject validate
											if isValidate {
												finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
											}

											return finalScriptContext, nil
										}

									}
								}

							} else if stageStatusMap.Get(stageId).IsSkipped {
								// If the stage is skipped, throw an error since it's a
								// dependency but it's skipped means it's trigger resolved to false.
								log.Warnln(logTag, ": ", fmt.Errorf("dependent stage with ID: %s skipped due to trigger resolving to `false`", stageId))

								// We will need to skip this stage as well.
								shouldSkipStage = true
								break

							} else {
								completedStages += 1
							}
						}

						if shouldSkipStage {
							break
						}

						// If all stages are done, then stop the loop
						if completedStages == len(dependencies) {
							break
						}
						if time.Since(start) > 60*time.Second {
							scriptContext.Logs = stageLogTracker.value

							// Capture stage took and set error status
							timeTook = int(time.Since(start).Milliseconds())
							var id string
							if pipeline.ID != nil {
								id = *pipeline.ID
							}

							finalScriptContext := globalScriptContext.GetMap()

							// Inject logs
							finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

							// Inject validate
							if isValidate {
								finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
							}

							return finalScriptContext, &Error{
								Err:  errors.New("pipeline timeout " + id),
								Code: http.StatusRequestTimeout,
							}
						}
					}
				}

				// Skip if one of the dependent stages skipped as well.
				if shouldSkipStage {
					// Show a warning and mark current stage as skipped.
					log.Warnln(logTag, ": skipping stage: `", *mainStageId, "` since one or more of dependent stages were skipped!")
					stageStatusMap.PutSkippedStatus(*mainStageId, true)
					continue
				}

				// resolved the dependencies, execute the script now
				log.Debug(logTag, ": executing stage: ", *mainStageId)
				shouldStopExecution, err := executeStage(
					mainStage,
					&stageStatusMap,
					&stageLogTracker,
					&globalScriptContext,
					&rsAPIContext,
					&cachedRequestContext,
					stageErrorTracker,
					pipelineExecutionContext,
					&stageChanges,
					&stagesInvokedMap,
					pipeline,
					req,
					stageDiffWg,
					bgScriptWg,
					&bgStagesToTime,
					shouldContinueInBg,
				)
				log.Debug(logTag, ": execution completed at: ", *mainStageId)
				if err != nil {
					scriptContext.Logs = stageLogTracker.value

					finalScriptContext := globalScriptContext.GetMap()

					// Inject logs
					finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

					// Inject validate
					if isValidate {
						finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
					}

					return finalScriptContext, err
				}
				if shouldStopExecution {
					break
				}
			}
		}

		// Record Cache and Analytics
		for _, pipeline := range *pipeline.Stages {
			if pipeline.Use != nil {
				switch *pipeline.Use {
				case UseCache:
					{
						executeRecordCacheStage(cachedRequestContext.Get(), scriptContext.Environments, globalScriptContext.value)
					}
				case RecordAnalytics:
					{
						contextWithQueryId, _ := executeRecordAnalyticsStage(globalScriptContext.value, scriptContext.Environments, &rsAPIContext)
						globalScriptContext.Put(contextWithQueryId, false)
					}
				}
			}
		}
	}
	// check error in go routines
	select {
	case stageError := <-stageErrorTracker:
		scriptContext.Logs = stageLogTracker.value

		// Capture took
		timeTook = int(time.Since(start).Milliseconds())

		finalScriptContext := globalScriptContext.GetMap()

		// Inject logs
		finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

		// Inject validate
		if isValidate {
			finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
		}

		return finalScriptContext, &stageError
	default:
	}

	var finalScriptContext = globalScriptContext.GetMap()

	// Capture final time took
	timeTook = int(time.Since(start).Milliseconds())

	log.Debug(logTag, ": time took for pipeline: ", timeTook)

	// If it's a validate call, add diff stages in the context
	// so that can be returned.
	if isValidate || isDebug {
		finalScriptContext = injectStageChanges(finalScriptContext, scriptContextBytes, stageChanges, parseDiffs, &timeTook, stageDiffWg, validateId)
	}

	// Inject logs
	finalScriptContext["console_logs"] = MergeStageLogs(stageLogTracker.value)

	// Record Analytics
	return finalScriptContext, nil
}

// injectStageChanges will inject the stage changes
func injectStageChanges(finalScriptContext map[string]interface{}, scriptContextBytes []byte, stageChanges []*StageChange, parseDiffs bool, timeTook *int, contextDiffWg *sync.WaitGroup, validateId string) map[string]interface{} {
	if parseDiffs {
		stringedContext := string(scriptContextBytes)
		dummyPipelineLog := PipelineLog{
			Context:      &stringedContext,
			StageChanges: new([]StageChange),
		}
		updateLogWithStages(&dummyPipelineLog, &stageChanges, true, contextDiffWg)

		// Marshal the log
		logInBytes, _ := json.Marshal(dummyPipelineLog)
		logInBytes, err := parseContextDiffs(logInBytes)
		if err != nil {
			log.Warnln(logTag, "error while parsing context diffs, ", err)
		}

		// Unmarshal into pipeline logs
		var pipelineMap map[string]interface{}
		err = json.Unmarshal(logInBytes, &pipelineMap)
		if err != nil {
			log.Warnln(logTag, "error while unmarshalling dummy pipeline, ", err)
		}

		finalScriptContext["stageChanges"] = pipelineMap["stageChanges"]
	} else {
		finalScriptContext["stageChanges"] = stageChanges
	}

	finalScriptContext["took"] = *timeTook

	// Register the validateId and inject it in the response
	if validateId != "" {
		Instance().validateSession.Register(validateId)
		finalScriptContext["id"] = validateId
	}

	return finalScriptContext
}

type Error struct {
	Err  error
	Code int
}

// appendStageIdToErr will append the stage ID to the
// passed error and return it.
func appendStageIdToErr(errPassed error, stageId string) error {
	return fmt.Errorf("%s: from stage with ID: %s", errPassed.Error(), stageId)
}

// extracts the envs from index
func getIndicesFromEnvs(envs map[string]interface{}) []string {
	indices := []string{}
	indicesInterface, ok := envs["index"].([]interface{})
	if ok {
		for _, index := range indicesInterface {
			indexAsString, ok := index.(string)
			if ok {
				indices = append(indices, indexAsString)
			}
		}
	}
	return indices
}

func updateLogWithStages(pipelineLog *PipelineLog, stageChanges *[]*StageChange, shouldUpdateLog bool, contextDiffWg *sync.WaitGroup) {
	if !shouldUpdateLog {
		return
	}

	// Wait for context diffs to be done
	startTime := time.Now()
	contextDiffWg.Wait()
	log.Debug("Time waited for diffs to be done: ", int(time.Since(startTime).Milliseconds()))

	for _, stageChange := range *stageChanges {
		*pipelineLog.StageChanges = append(*pipelineLog.StageChanges, *stageChange)
	}
}

// startUpdateLogs will start the update logs process that also
// waits for the logs to update and then returns.
func startUpdateLogs(pipelineLog *PipelineLog, stageChanges *[]*StageChange, shouldUpdateLog bool, contextDiffWg *sync.WaitGroup, logWg *sync.WaitGroup) {
	log.Debug(logTag, ": wg now: ", pretty.Formatter(contextDiffWg))
	go func() {
		updateLogWithStages(pipelineLog, stageChanges, shouldUpdateLog, contextDiffWg)
		logWg.Done()
	}()
}

func executePreBuiltStage(
	stage ESPipelineStage,
	globalScriptContext *GlobalScriptContext,
	rsAPIRequest *ReactiveSearchQueryContext,
	cachedRequestContext *CachedRequestContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time,
	req *http.Request,
	hasBoostStage bool,
) ([]byte, bool, *Error) {
	scriptContextInBytes := globalScriptContext.Get()
	contextMap := globalScriptContext.GetMap()
	inputVariables := make(map[string]interface{})
	envVariables, ok := contextMap["envs"].(map[string]interface{})
	if ok {
		for k, v := range envVariables {
			inputVariables[k] = v
		}
	}
	// apply user properties from context
	for k, v := range contextMap {
		if !util.Contains(CONTEXT_RESERVED_KEYS, k) {
			inputVariables[k] = v
		}
	}
	parsedInputs, err := getInputValuesFromContext(stage.Inputs, inputVariables)
	if err != nil {
		return scriptContextInBytes, false, &Error{
			Err: err,
		}
	}
	// pre-built stage
	switch *stage.Use {
	// translates the RS API request body to ES query
	case ReactiveSearchQuery:
		return executeReactivesearchStage(
			stage,
			parsedInputs,
			globalScriptContext,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime,
			hasBoostStage,
		)
	case SearchRelevancy:
		return executeSearchRelevancyStage(
			stage,
			parsedInputs,
			scriptContextInBytes,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime)
	case SearchboxPreferences:
		return executeSearchPreferencesStage(
			stage,
			parsedInputs,
			scriptContextInBytes,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime)
	case RecordClick:
		return executeRecordClickStage(
			stage,
			req,
			globalScriptContext,
			scriptEnvs,
			async,
			startTime)
	case RecordConversion:
		return executeRecordConversionStage(
			stage,
			req,
			globalScriptContext,
			scriptEnvs,
			async,
			startTime)
	case RecordFavorite:
		return executeRecordFavoriteStage(
			stage,
			req,
			globalScriptContext,
			scriptEnvs,
			async,
			startTime)
	case RecordSaveSearch:
		return executeRecordSaveSearch(
			stage,
			req,
			globalScriptContext,
			scriptEnvs,
			async,
			startTime)
	case Boost:
		return executeBoostStage(
			stage,
			parsedInputs,
			scriptContextInBytes,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime)
	// Queries the Elasticsearch BE
	case ElasticSearchQuery:
		return executeElasticsearchStage(
			stage,
			parsedInputs,
			globalScriptContext,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime,
		)
	case HttpRequest:
		return executeHTTPRequestStage(
			stage,
			parsedInputs,
			globalScriptContext,
			scriptEnvs,
			async,
			startTime,
		)
	case MongoDBQuery:
		return executeMongoDBStage(
			stage,
			parsedInputs,
			globalScriptContext,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime,
		)
	case SolrQuery:
		return executeSolrStage(
			stage,
			parsedInputs,
			globalScriptContext,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime,
		)
	case ZincQuery:
		return executeZincStage(
			stage,
			parsedInputs,
			globalScriptContext,
			rsAPIRequest,
			scriptEnvs,
			async,
			startTime,
		)
	case UseCache:
		return executeUseCacheStage(
			cachedRequestContext,
			scriptContextInBytes,
			scriptEnvs,
			startTime,
			rsAPIRequest,
		)
	case PromoteResults:
		return executePromoteResult(stage, parsedInputs, scriptContextInBytes, rsAPIRequest)
	case HideResults:
		return executeHideResult(stage, parsedInputs, scriptContextInBytes)
	case CustomData:
		return executeCustomData(stage, parsedInputs, scriptContextInBytes)
	case ReplaceSearchTerm:
		return executeReplaceSearch(stage, parsedInputs, scriptContextInBytes)
	case AddFilter:
		return executeAddFilter(stage, parsedInputs, scriptContextInBytes)
	case RemoveWords:
		return executeRemoveWords(stage, parsedInputs, scriptContextInBytes)
	case ReplaceWords:
		return executeReplaceWords(stage, parsedInputs, scriptContextInBytes)
	case KnnResponse:
		return executeKnn(stage, rsAPIRequest, parsedInputs, scriptContextInBytes)
	case OpenAIEmbeddings:
		return executeOpenAIEmbeddingsStage(stage, parsedInputs, globalScriptContext, rsAPIRequest, scriptEnvs, async, startTime)
	case OpenAIEmbeddingsIndex:
		return executeOpenAIEmbeddingsIndexStage(stage, parsedInputs, globalScriptContext, rsAPIRequest, scriptEnvs, async, startTime)
	case AIAnswer:
		return executeAIAnswerStage(stage, parsedInputs, globalScriptContext, rsAPIRequest, scriptEnvs, async, startTime)
	case ValidateStage:
		return executeValidateStage(stage, req, globalScriptContext, rsAPIRequest, scriptEnvs, async, startTime)
	}
	return scriptContextInBytes, false, nil
}

// PipelineMatcher returns a mux matcher function that executes
// the trigger expression for the pipeline.
//
// The trigger will be extracted from the attached pipeline and
// accordingly executed.
func (p *ESPipelineDoc) PipelineMatcher(route ESPipelineRoutes) mux.MatcherFunc {
	return func(req *http.Request, match *mux.RouteMatch) bool {
		// If trigger is not present return true
		if p.Trigger == nil {
			return true
		}

		// Validate the time frame
		ok := validateTimeframe(*p)

		// If timeframe validation failed, return false
		if !ok {
			return ok
		}

		// Save the category and ACL in context since that will
		// be used to verify if the request is an indexing one.
		routeCategory := category.Pipelines
		if route.Classify != nil && route.Classify.Category != nil {
			routeCategory = *route.Classify.Category
		}
		ctx := category.NewContext(req.Context(), &routeCategory)
		req = req.WithContext(ctx)

		// Set the ACL based on the route
		if route.Classify != nil && route.Classify.ACL != nil {
			routeACL := *route.Classify.ACL
			ctx := acl.NewContext(req.Context(), &routeACL)
			req = req.WithContext(ctx)
		}

		status, err := runTrigger(req.Context(), req, *p)
		if err != nil {
			log.Warnln(logTag, "error occurred while validating trigger from matcher: ", err)
			return false
		}

		return status
	}
}

// Evaluates the dynamic inputs variables from global script context
func getInputValuesFromContext(inputs *string, context map[string]interface{}) (*string, error) {
	if inputs == nil {
		return inputs, nil
	}
	resolvedInputs := *inputs

	// The older implementation was iterating through all the key values
	// in the context passed and checking if the {{<key>}} is present
	// in the string and accordingly replacing it with a value.
	//
	// The newer implementation goes around in the other way. It iterates
	// over the string to find out parts of
	// string that have the patter {{.*+}} and it will extract that part.
	// After extracting the part, it will extract the string between the
	// moustache and then accordingly go through it.
	//
	// Assumptions (or technical reasoning) are:
	// - if a string has multiple nests, say `a.b.c` then a and b should be
	// of type map or an error will be thrown.

	patternForDynamic := regexp.MustCompile(`{{{?[^,:]+}?}}`)
	matches := patternForDynamic.FindAllString(resolvedInputs, -1)

	for _, inputValueToReplace := range matches {
		// Value would be in the format {{<str>}}
		// Remove the moustaches.
		nakedValue := regexp.MustCompile("{{{?|}?}}").ReplaceAllString(inputValueToReplace, "")

		// Check if we need to preserve type.
		shouldPreserveType := false
		if strings.HasPrefix(inputValueToReplace, "{{{") {
			shouldPreserveType = true
		}

		log.Debugln(logTag, ": value to find in context for dynamic input: ", nakedValue)

		// We need to split it based on dots or so.
		valueToReplaceWith, nestedFindErr := util.GetNestedValueFromContext(nakedValue, context)

		if nestedFindErr != nil {
			log.Warnln(logTag, ": error while resolving dynamic inputs: ", nestedFindErr)
			return &resolvedInputs, nestedFindErr
		}

		// Marshal the value into a string
		valueAsByte, marshalErr := json.Marshal(valueToReplaceWith)
		if marshalErr != nil {
			log.Warnln(logTag, ": error while marshalling value for dynamic input for key: ", inputValueToReplace)
			return &resolvedInputs, marshalErr
		}
		valueAsString := string(valueAsByte)

		inputStringToReplaceWith := inputValueToReplace

		// If type doesn't have to be preserved, we need to make it a string.
		if shouldPreserveType {
			inputStringToReplaceWith = fmt.Sprintf(`"%s"`, inputStringToReplaceWith)
		}

		// Finally, replace the string with the value in the resolvedInputs
		// string.
		//
		// First try to replace with the quotes wrapping the input value and then
		// without the quotes.
		//
		// This is important because sometimes the value can be `x/{{something}}`
		// and sometimes the value can be `"{{something}}"`.

		if !shouldPreserveType {
			// Remove quotes from the value
			unquotedValueAsString, unquoteErr := strconv.Unquote(valueAsString)
			if unquoteErr != nil {
				log.Warnln(logTag, ": error while unquoting marshalled value for dynamic input key: ", inputValueToReplace, ", with error: ", unquoteErr)
				// No need to throw error since there can be times
				// when type doesn't have quotes at all.
				unquotedValueAsString = valueAsString
			}

			resolvedInputs = strings.Replace(resolvedInputs, inputValueToReplace, unquotedValueAsString, -1)
		} else {
			resolvedInputs = strings.Replace(resolvedInputs, inputStringToReplaceWith, valueAsString, -1)
			resolvedInputs = strings.Replace(resolvedInputs, inputValueToReplace, valueAsString, -1)
		}

	}

	return &resolvedInputs, nil
}
