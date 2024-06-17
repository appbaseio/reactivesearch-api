package pipelines

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/plugins/analytics"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/ghodss/yaml"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
)

// Add the pipeline the the database after parsing it.
func (p *Pipelines) postPipeline(req *http.Request, w http.ResponseWriter, pipelineBody ESPipelineDoc) {
	// Validate the pipeline body.
	validateErr := p.validatePipelineBody(pipelineBody)
	if validateErr != nil {
		errMsg := fmt.Sprint("invalid pipeline body passed, ", validateErr)
		log.Warnln(logTag, errMsg)
		telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
		return
	}

	// Validate the size requirements for the body
	// If the user has exceeded the max number of pipelines that they can create
	// for their plan, then an error will be thrown.
	size, err := p.es.getPipelinesSize(req.Context())
	if err != nil {
		log.Errorln(logTag, ": error while getting pipelines size, ", err)
		telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
		return
	}

	if util.GetTier() != nil {
		if IsPipelineLimitExceeded(*size) {
			log.Warnln(logTag, " maximum size exceeded for user")
			errMsg := fmt.Sprint("You have reached the maximum pipelines allowed for the ", util.GetTier().String(), "plan")
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusPaymentRequired)
			return
		}
	}

	// If the pipeline ID is passed, check if it is already
	// present, if it is, then we need to raise an error.
	// If it is not passed, we need to assign a new one to the
	// pipeline.
	if pipelineBody.ID != nil && *pipelineBody.ID != "" {
		// Verify the ID is not already present
		pipeline, _ := GetPipelineAndLocFromCache(*pipelineBody.ID)
		if pipeline != nil {
			// Raise error that the pipeline already exists.
			errMsg := fmt.Sprintf("pipeline with ID: `%s` already exists", *pipelineBody.ID)
			log.Warnln(errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusConflict)
			return
		}

		// Make sure the ID is urlsafe
		//
		// Regex is picked up from this stackoverflow link
		// https://stackoverflow.com/a/24419159/8091283
		isUrlSafe, _ := regexp.MatchString(`^[a-zA-Z0-9_-]*$`, *pipelineBody.ID)
		if !isUrlSafe {
			errMsg := fmt.Sprintf("pipeline ID passed is not URL safe `%s`", *pipelineBody.ID)
			log.Warnln(errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}
	} else {
		// Assign a new unique ID to the pipeline
		pipelineID := uuid.New().String()
		pipelineBody.ID = &pipelineID
	}

	// Add created at for the pipeline
	createdAt := time.Now().Unix()
	pipelineBody.CreatedAt = &createdAt

	// Set the enabled value if it's not passed.
	if pipelineBody.Enabled == nil {
		defaultEnabled := true
		pipelineBody.Enabled = &defaultEnabled
	}

	// Make the current pipeline as the first version of
	// the pipeline.
	pipelineBodyMarshalled, pipelineMarshalErr := json.Marshal(pipelineBody)
	if pipelineMarshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling pipeline to save it as first version: ", pipelineMarshalErr)
		log.Errorln(logTag, errMsg)
		telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
		return
	}
	pipelineAsString := string(pipelineBodyMarshalled)
	firstVersionId := 1
	firstVersionDescription := "Init pipeline creation"

	firstVersion := Version{
		Content:     &pipelineAsString,
		Version:     &firstVersionId,
		Description: &firstVersionDescription,
	}
	pipelineBody.Versions = &[]Version{firstVersion}

	pipelineBody.LiveVersion = &firstVersionId

	// Store the dco into ES
	esErr := p.es.createPipeline(req.Context(), *pipelineBody.ID, pipelineBody)
	if esErr != nil {
		log.Errorln(logTag, ": error while saving the doc to ES, ", err)
		telemetry.WriteBackErrorWithTelemetry(req, w, esErr.Error(), http.StatusInternalServerError)
		return
	}

	// Invoke ACCAPI
	// We're sending the request body as the final parsed struct to be stored in cache
	// The reason for doing is to avoid recreating the properties like `id` and `createdAt` that
	// can lead to inconsistency.
	//
	// Only update local state when proxy API has not been called
	// If proxy API would get called then it would automatically update the
	// state for all machines
	// Updating the local state again can cause insconsistency issues
	if util.ShouldProxyToACCAPI() {
		// Convert the ESPipelineDoc to map
		pipelineMap, _ := PipelineToMap(pipelineBody)

		res, err := util.ProxyACCAPI(util.ProxyConfig{
			Method: http.MethodPost,
			URL:    "/_pipeline",
			Body:   pipelineMap, // forward body
		})
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Failed to update all nodes, return error response
		if res != nil {
			log.Errorln(logTag, ":", "error encountered creating pipeline")
			bodyBytes, err := ioutil.ReadAll(res.Body)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, bodyBytes, res.StatusCode)
			return
		}
	} else {
		// Update cache
		AddPipelineToCache(pipelineBody)
	}

	// Remove the originalConfig before marshalling
	pipelineBody.OriginalConfig = nil
	marshalledPipeline, err := json.Marshal(pipelineBody)
	if err != nil {
		log.Errorln(logTag, ": error while marshalling response, ", err)
		telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
		return
	}

	util.WriteBackRaw(w, marshalledPipeline, http.StatusCreated)
}

func (p *Pipelines) getPipelineSchema() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		util.WriteBackRaw(w, p.pipelineSchema, http.StatusOK)
	}
}

// Create the pipeline using the passed files.
func (p *Pipelines) postFormPipeline() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Support JSON body if the call is a local call
		// If it is a local call, we will expect JSON body of
		// the ESPipelineDoc that we will un-marshall and store in the
		// cache.
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Unmarshal the pipeline body.
			var pipelineBody ESPipelineDoc

			// Read the body
			reqBody, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Warnln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
				return
			}

			defer req.Body.Close()

			unmarshallErr := json.Unmarshal(reqBody, &pipelineBody)
			if unmarshallErr != nil {
				errMsg := fmt.Sprint("couldn't unmarshall request body: ", err)
				log.Warnln(logTag, errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}

			// Update cache
			AddPipelineToCache(pipelineBody)

			// Remove the originalConfig before marshalling
			pipelineBody.OriginalConfig = nil
			marshalledPipeline, err := json.Marshal(pipelineBody)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, marshalledPipeline, http.StatusCreated)
			return
		}

		pipelineBody, err := p.parsePipelineFile(req)
		if err != nil {
			code := http.StatusInternalServerError
			if err.Code != 0 {
				code = err.Code
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Err.Error(), code)
			return
		}

		// Rest of the creation will be handled by the older create pipeline
		// method
		p.postPipeline(req, w, *pipelineBody)
	}
}

// Validates and renders the pipeline output using the passed files.
func (p *Pipelines) validateFormPipeline() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		parseDiffs := req.URL.Query().Get("verbose")
		parseDiffBool := true
		if parseDiffs == "false" {
			parseDiffBool = false
		}

		pipelineBody, err := p.parsePipelineFile(req)
		if err != nil {
			code := http.StatusInternalServerError
			if err.Code != 0 {
				code = err.Code
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Err.Error(), code)
			return
		}
		var requestBodyScript rules.ScriptRequest
		// read request
		requestBody := req.PostFormValue("request")
		if requestBody != "" {
			err2 := json.Unmarshal([]byte(requestBody), &requestBodyScript)
			if err2 != nil {
				errMsg := fmt.Sprint("error while reading request key, is it a valid JSON?: ", err2.Error())
				log.Warnln(logTag, "error while reading request, ", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}
		}
		var body map[string]interface{}
		err4 := json.Unmarshal([]byte(requestBodyScript.Body), &body)
		if err4 != nil {
			log.Errorln(logTag, ":", err4)
		}
		Pipeline := decodePipelineScripts(*pipelineBody)
		envsParsed := Pipeline.getPipelineEnvironments(req, []byte(requestBodyScript.Body))

		// We need to hardcode the category as the category of the pipeline
		// for all validate requests if the category is not already set.
		categoryPassed, isPresent := envsParsed["category"]
		if !isPresent || categoryPassed == "" {
			// Use the pipeline passed to use the category defined
			// there else just default to `reactivesearch`.
			categoryToSet := "reactivesearch"
			if pipelineBody != nil && pipelineBody.Routes != nil &&
				len(*pipelineBody.Routes) != 0 &&
				(*pipelineBody.Routes)[0].Classify != nil &&
				(*pipelineBody.Routes)[0].Classify.Category != nil {
				categoryToSet = (*pipelineBody.Routes)[0].Classify.Category.String()
			}

			envsParsed["category"] = categoryToSet
		}

		// execute the pipeline
		pipelineExecutionContext := PipelineExecutionContext{
			envs: envsParsed,
			request: PipelineExecutionRequest{
				Body:    []byte(requestBodyScript.Body),
				Headers: requestBodyScript.Headers,
			},
		}
		scriptTimeTracker := time.Now()
		scriptContext, pipelineErr := Pipeline.executePipeline(pipelineExecutionContext, req.Context(), false, nil, true, parseDiffBool, false, req)
		if pipelineErr != nil {
			code := http.StatusInternalServerError
			if pipelineErr.Code != 0 {
				code = pipelineErr.Code
			}

			msg := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    code,
					"status":  http.StatusText(code),
					"message": pipelineErr.Err.Error(),
				},
			}

			// Set the msg as the response.body field.
			marshalledResponseBody, _ := json.Marshal(msg)
			scriptContext["response"] = map[string]interface{}{
				"body":    string(marshalledResponseBody),
				"code":    code,
				"headers": map[string]interface{}{},
			}

			// Marshal the scriptContext
			scriptContextInBytes, err2 := json.Marshal(scriptContext)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusInternalServerError)
				return
			}

			util.WriteBackRaw(w, scriptContextInBytes, code)
			return
		}
		scriptContext["took"] = time.Since(scriptTimeTracker).Milliseconds()

		// Replace the body field with the original body.
		scriptContext["request"].(map[string]interface{})["body"] = requestBodyScript.Body

		scriptContextInBytes, err2 := json.Marshal(scriptContext)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, scriptContextInBytes, http.StatusOK)
	}
}

// Put pipeline using the ID and update based on the body passed
// by the user
func (p *Pipelines) putPipeline(req *http.Request, rw http.ResponseWriter, pipelineBody ESPipelineDoc, oldPipeline ESPipelineDoc, pipelineID string) {
	// Validate the pipeline
	validateErr := p.validatePipelineBody(pipelineBody)
	if validateErr != nil {
		errMsg := fmt.Sprint("invalid pipeline body passed, ", validateErr)
		log.Warnln(logTag, errMsg)
		telemetry.WriteBackErrorWithTelemetry(req, rw, errMsg, http.StatusBadRequest)
		return
	}

	// Add a special case for putting pipelines where user cannot pass the ID
	// in the pipeline file.
	if pipelineBody.ID != nil && *pipelineBody.ID != pipelineID {
		errMsg := fmt.Sprint("ID passed in pipeline file should be same as the original ID.")
		log.Warnln(logTag, errMsg)
		telemetry.WriteBackErrorWithTelemetry(req, rw, errMsg, http.StatusUnprocessableEntity)
		return
	}

	// The new es pipeline will not have the createdAt, so copy from old one
	pipelineBody.CreatedAt = oldPipeline.CreatedAt

	// If the enabled field is nil in the new body, copy from old one
	if pipelineBody.Enabled == nil {
		pipelineBody.Enabled = oldPipeline.Enabled
	}

	// Set the live version from the old pipeline as well
	pipelineBody.LiveVersion = oldPipeline.LiveVersion

	// Add the passed rule into the body, in case the user passed it.
	pipelineBody.ID = &pipelineID

	// Add updatedat
	updatedAt := time.Now().Unix()
	pipelineBody.UpdatedAt = &updatedAt

	// Update the body in ES
	esErr := p.es.updatePipeline(req.Context(), pipelineID, pipelineBody)
	if esErr != nil {
		log.Errorln(logTag, ": error occurred while writing body to ES, ", esErr)
		telemetry.WriteBackErrorWithTelemetry(req, rw, esErr.Error(), http.StatusInternalServerError)
		return
	}

	// Only update local state when proxy API has not been called
	// If proxy API would get called then it would automatically update the
	// state for all machines
	// Updating the local state again can cause insconsistency issues
	if util.ShouldProxyToACCAPI() {
		// Convert the ESPipelineDoc to map
		pipelineMap, err := PipelineToMap(pipelineBody)
		if err != nil {
			log.Errorln(logTag, ": error while converting body to map, ", err)
			telemetry.WriteBackErrorWithTelemetry(req, rw, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := util.ProxyACCAPI(util.ProxyConfig{
			Method: http.MethodPut,
			URL:    "/_pipeline/" + pipelineID,
			Body:   pipelineMap,
		})

		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, rw, err.Error(), http.StatusInternalServerError)
			return
		}
		// Failed to update all nodes, return error response
		if res != nil {
			log.Errorln(logTag, ":", "error encountered updating pipeline")
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, rw, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	} else {
		// Update Cache
		ok := UpdatePipelineInCache(pipelineID, pipelineBody)
		if !ok {
			msg := "Error encountered while updating the pipeline"
			log.Errorln(logTag, ":", msg)
			telemetry.WriteBackErrorWithTelemetry(req, rw, msg, http.StatusInternalServerError)
			return
		}
	}

	util.WriteBackMessage(rw, "Pipeline updated succesfully", http.StatusOK)
}

// Put pipeline using the ID and files with multiform data support
func (p *Pipelines) putFormPipeline() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		// If it is a local request, this is just to update the proxies.
		// Just update the pipeline in the cache and return.
		// NOTE: We will parse the body as JSON since proxy update calls will
		// pass an interface.
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Unmarshal the pipeline body.
			var pipelineBody ESPipelineDoc

			// Read the body
			reqBody, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Warnln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, rw, "Can't read request body", http.StatusBadRequest)
				return
			}

			defer req.Body.Close()

			unmarshallErr := json.Unmarshal(reqBody, &pipelineBody)
			if unmarshallErr != nil {
				errMsg := fmt.Sprint("couldn't unmarshall request body: ", err)
				log.Warnln(logTag, errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, rw, errMsg, http.StatusBadRequest)
				return
			}

			// Update Cache
			ok := UpdatePipelineInCache(pipelineID, pipelineBody)
			if !ok {
				msg := "Error encountered while updating the pipeline"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, rw, msg, http.StatusInternalServerError)
				return
			}

			util.WriteBackMessage(rw, "Pipeline is updated successfully", http.StatusOK)
			return
		}

		// Verify pipeline ID is valid by getting the pipeline
		// from cache.
		pipeline, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipeline == nil {
			errMsg := fmt.Sprint(pipelineID, ": invalid pipeline ID passed")
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, rw, errMsg, http.StatusNotFound)
			return
		}

		esPipeline, err := p.parsePipelineFile(req)
		if err != nil {
			code := http.StatusInternalServerError
			if err.Code != 0 {
				code = err.Code
			}
			telemetry.WriteBackErrorWithTelemetry(req, rw, err.Err.Error(), code)
			return
		}

		// Validate the pipeline and update it in cache
		p.putPipeline(req, rw, *esPipeline, *pipeline, pipelineID)
	}
}

// Delete the pipeline by using the passed ID
func (p *Pipelines) deletePipeline() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		// Determine whether to just update locally.
		// If true, just update the cache and do nothing
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update Cache
			ok := DeletePipelineFromCache(pipelineID)
			if !ok {
				msg := "Error encountered while deleting the pipeline"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}

			util.WriteBackMessage(w, "Pipeline is deleted successfully", http.StatusOK)
			return
		}

		// Delete the pipeline from ES
		err := p.es.deletePipeline(req.Context(), pipelineID)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}

		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Invoke ACCAPI
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodDelete,
				URL:    "/_pipeline/" + pipelineID,
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered deleting pipeline")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update Cache
			ok := DeletePipelineFromCache(pipelineID)
			if !ok {
				msg := "Error encountered while deleting the pipeline"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
		}

		util.WriteBackMessage(w, "Pipeline is deleted successfully", http.StatusOK)

	}
}

// Get pipeline based on the passed ID
func (p *Pipelines) getPipeline() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		pipelineID := vars["id"]

		// Get the pipeline from cache
		pipeline, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipeline == nil {
			errMsg := fmt.Sprint("pipeline not found for ID: ", pipelineID)
			telemetry.WriteBackErrorWithTelemetry(r, rw, errMsg, http.StatusNotFound)
			return
		}

		// Since the pipeline is fetched, we will return the original config
		// and not the pipeline content that we have stored.
		configOut := getConfigOut(*pipeline, true)

		marshalledPipeline, err := json.Marshal(configOut)
		if err != nil {
			telemetry.WriteBackErrorWithTelemetry(r, rw, err.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(rw, marshalledPipeline, http.StatusOK)
	}
}

// Get all pipelines in the database
func (p *Pipelines) getPipelines() http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		allPipelines := GetPipelinesFromCache()

		if len(allPipelines) == 0 {
			allPipelines = make([]ESPipelineDoc, 0)
		}

		// We need to return original config only
		originalPipelineFiles := make([]ConfigOut, 0)
		for _, pipeline := range allPipelines {
			configOut := getConfigOut(pipeline, true)
			originalPipelineFiles = append(originalPipelineFiles, configOut)
		}

		marshalledPipelines, err := json.Marshal(originalPipelineFiles)
		if err != nil {
			log.Warnln(logTag, ": error while marshalling all pipelines, ", err)
			telemetry.WriteBackErrorWithTelemetry(r, rw, err.Error(), http.StatusInternalServerError)
			return
		}

		// Else return the response
		util.WriteBackRaw(rw, marshalledPipelines, http.StatusOK)
	}
}

// getScriptRef returns the script content for the passed scriptRef
// in the pipeline passed using the ID.
func (p *Pipelines) getScriptRef() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		// Extract the key from the query
		scriptRefKey := req.URL.Query().Get("key")

		if scriptRefKey == "" {
			errMsg := "key cannot be empty"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Get the pipeline from cache
		pipeline, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipeline == nil {
			errMsg := fmt.Sprint("pipeline not found for ID: ", pipelineID)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// Get the live version of the pipeline if it is present
		if pipeline.LiveVersion != nil && pipeline.Versions != nil {
			pipelineVersion, _ := GetPipelineVersion(*pipeline, *pipeline.LiveVersion)
			if pipelineVersion != nil {
				var versionedPipeline ESPipelineDoc
				unmarshalErr := json.Unmarshal([]byte(*pipelineVersion.Content), &versionedPipeline)
				if unmarshalErr != nil {
					errMsg := fmt.Sprint("could not unmarshal version of pipeline: ", unmarshalErr.Error())
					log.Warnln(logTag, ": ", errMsg)
					telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
					return
				}

				decodedPipeline := decodePipelineScripts(versionedPipeline)
				pipeline = &decodedPipeline
			}
		}

		var matchedStage *ESPipelineStage
		var fallbackMatchedStage *ESPipelineStage

		// Determine if the fallback should be matched
		// Fallback will be matched only if the script ref has an extension
		//
		// shouldMatchFallback will determine if a fallback should be matched
		keyExtension := filepath.Ext(scriptRefKey)
		extStrippedKey := ""
		shouldMatchFallback := false

		// If the extension is present, determine the fallback key
		// to match and enable fallback matching.
		if keyExtension != "" {
			shouldMatchFallback = true
			extStrippedKey = strings.TrimSuffix(scriptRefKey, keyExtension)
		}

		// Find the scriptRef using the passed key
		for _, pipelineStage := range *pipeline.Stages {
			// If scriptRef is not present, continue
			if pipelineStage.ScriptRef == nil {
				continue
			}

			// Try to see if the key is matched
			if *pipelineStage.ScriptRef == scriptRefKey {
				matchedStage = &pipelineStage
				break
			}

			// Match fallback if it is enabled
			if shouldMatchFallback && *pipelineStage.ScriptRef == extStrippedKey {
				fallbackMatchedStage = &pipelineStage
			}
		}

		if matchedStage == nil && fallbackMatchedStage == nil {
			// scriptRef was invalid
			errMsg := fmt.Sprint(scriptRefKey, ": invalid script ref key passed")
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// If matched stage is nil and fallback is not, replace matched stage with
		// fallback
		// If code executes till this point, at least one of the above is not nil
		if matchedStage == nil && fallbackMatchedStage != nil {
			matchedStage = fallbackMatchedStage
		}

		// Else marshal the script content and return it.
		contentType := "js"
		scriptRefContent := ConfigOut{
			ID:      &pipelineID,
			Content: matchedStage.Script,
			Type:    &contentType,
			Key:     &scriptRefKey,
		}

		// Marshal the body
		marshalledContent, err := json.Marshal(scriptRefContent)
		if err != nil {
			errMsg := fmt.Sprint("could not marshal the script ref content, ", err)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Finally write back the marshalled content
		util.WriteBackRaw(w, marshalledContent, http.StatusOK)
	}
}

// getVersionScriptRef returns the versions script content for the passed scriptRef
// in the pipeline passed using the ID.
func (p *Pipelines) getVersionScriptRef() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		pipelineVersionID := vars["version_id"]
		versionAsInt, convertErr := strconv.Atoi(pipelineVersionID)
		if convertErr != nil {
			errMsg := fmt.Sprint("invalid version passed for pipeline: ", convertErr)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Extract the key from the query
		scriptRefKey := req.URL.Query().Get("key")

		if scriptRefKey == "" {
			errMsg := "key cannot be empty"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Get the pipeline from cache
		pipeline, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipeline == nil {
			errMsg := fmt.Sprint("pipeline not found for ID: ", pipelineID)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// If the version passed is 1 and the pipeline doesn't have any
		// versions defined, we can go ahead with the pipeline content directly.
		pipelineVersion, _ := GetPipelineVersion(*pipeline, versionAsInt)
		if pipelineVersion == nil {
			errMsg := fmt.Sprintf("version with number: `%d` not found for pipeline", versionAsInt)
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Get the version of the pipeline into a pipeline object
		var versionedPipeline ESPipelineDoc
		unmarshalErr := json.Unmarshal([]byte(*pipelineVersion.Content), &versionedPipeline)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("could not unmarshal version of pipeline: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		decodedPipeline := decodePipelineScripts(versionedPipeline)
		pipeline = &decodedPipeline

		var matchedStage *ESPipelineStage
		var fallbackMatchedStage *ESPipelineStage

		// Determine if the fallback should be matched
		// Fallback will be matched only if the script ref has an extension
		//
		// shouldMatchFallback will determine if a fallback should be matched
		keyExtension := filepath.Ext(scriptRefKey)
		extStrippedKey := ""
		shouldMatchFallback := false

		// If the extension is present, determine the fallback key
		// to match and enable fallback matching.
		if keyExtension != "" {
			shouldMatchFallback = true
			extStrippedKey = strings.TrimSuffix(scriptRefKey, keyExtension)
		}

		// Find the scriptRef using the passed key
		for _, pipelineStage := range *pipeline.Stages {
			// If scriptRef is not present, continue
			if pipelineStage.ScriptRef == nil {
				continue
			}

			// Try to see if the key is matched
			if *pipelineStage.ScriptRef == scriptRefKey {
				matchedStage = &pipelineStage
				break
			}

			// Match fallback if it is enabled
			if shouldMatchFallback && *pipelineStage.ScriptRef == extStrippedKey {
				fallbackMatchedStage = &pipelineStage
			}
		}

		if matchedStage == nil && fallbackMatchedStage == nil {
			// scriptRef was invalid
			errMsg := fmt.Sprint(scriptRefKey, ": invalid script ref key passed")
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// If matched stage is nil and fallback is not, replace matched stage with
		// fallback
		// If code executes till this point, at least one of the above is not nil
		if matchedStage == nil && fallbackMatchedStage != nil {
			matchedStage = fallbackMatchedStage
		}

		// Else marshal the script content and return it.
		contentType := "js"
		scriptRefContent := ConfigOut{
			ID:      &pipelineID,
			Content: matchedStage.Script,
			Type:    &contentType,
			Key:     &scriptRefKey,
		}

		// Marshal the body
		marshalledContent, err := json.Marshal(scriptRefContent)
		if err != nil {
			errMsg := fmt.Sprint("could not marshal the script ref content, ", err)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Finally write back the marshalled content
		util.WriteBackRaw(w, marshalledContent, http.StatusOK)
	}
}

// getLogsForPipelines returns the pipeline logs by fetching them and
// applying all the filters
func (p *Pipelines) getLogsForPipelines() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// Nothing to extact here, just call the getLogs method
		p.getLogs(rw, req, false)
	}
}

// getLogsForPipeline returns the logs for the pipeline using
// the passed pipeline ID.
//
// `category` is not allowed in this endpoint and if passed, will be
// ignored.
func (p *Pipelines) getLogsForPipeline() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		p.getLogs(rw, req, true)
	}
}

// getLogs returns the logs for pipeline and acts as the main entrypoint
// to fetch the logs for pipeline
func (p *Pipelines) getLogs(rw http.ResponseWriter, req *http.Request, pipelineSpecific bool) {

	// If pipeline specific, extract the pipeline ID
	pipelineID := ""
	if pipelineSpecific {
		pipelineID = mux.Vars(req)["id"]
	}

	// Extract the filters
	offset := req.URL.Query().Get("from")
	if offset == "" {
		offset = "0"
	}

	parsedOffset, err := strconv.Atoi(offset)
	if err != nil {
		errMsg := fmt.Errorf(`invalid value "%v" for query param "from"`, offset)
		log.Errorln(logTag, ": ", errMsg)
		util.WriteBackError(rw, err.Error(), http.StatusBadRequest)
		return
	}

	rangeParams := logs.RangeQueryParams(req.URL.Query())

	filter := req.URL.Query().Get("category")

	logsFilterConfig := logsFilter{
		Offset:     parsedOffset,
		StartDate:  rangeParams.StartDate,
		EndDate:    rangeParams.EndDate,
		Size:       rangeParams.Size,
		Filter:     filter,
		PipelineID: pipelineID,
	}

	sortBy := req.URL.Query().Get("sort_by")

	// If nothing is passed, set default value to timestamp_desc
	if sortBy == "" {
		sortBy = "timestamp_desc"
	}

	sortErr := parseSortBy(sortBy, &logsFilterConfig)
	if sortErr != nil {
		log.Warnln(logTag, ": error parsing sort_by: ", sortErr)
		telemetry.WriteBackErrorWithTelemetry(req, rw, sortErr.Error(), http.StatusBadRequest)
		return
	}

	raw, err := p.logEs.getPipelineLogs(req.Context(), logsFilterConfig)
	if err != nil {
		log.Errorln(logTag, ": error fetching logs :", err)
		util.WriteBackError(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	util.WriteBackRaw(rw, raw, http.StatusOK)
}

// getLogById returns the log for the passed ID. If no match is found, a
// 404 will be raised.
func (p *Pipelines) getLogById() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		logId := vars["id"]

		// ParseDiff flag
		parseDiffs := req.URL.Query().Get("verbose")
		parseDiffBool := true
		if parseDiffs == "false" {
			parseDiffBool = false
		}

		raw, err := p.logEs.getPipelineLogById(req.Context(), logId, parseDiffBool)
		if err != nil {
			log.Warnln(logTag, err.Err.Error())
			telemetry.WriteBackErrorWithTelemetry(req, rw, err.Err.Error(), err.Code)
			return
		}

		util.WriteBackRaw(rw, raw, http.StatusOK)
	}
}

// Validate the passed pipeline doc to make sure it follows the requirements
// for creating a pipeline.
// If not, an error will be returned which should be handled by the calling
// method.
func (p *Pipelines) validatePipelineBody(pipelineBody ESPipelineDoc) error {

	// Make sure that routes is not empty and if it's passed then at least one value
	// is there.
	if pipelineBody.Routes == nil || len(*pipelineBody.Routes) == 0 {
		return errors.New("routes is a required field, at least one route should be passed")
	}

	// If the routes are passed, make sure the path and method are passed for every route
	// because those are required params.
	// If we reach this code, the routes array is not empty and not nil.
	for position, route := range *pipelineBody.Routes {
		if route.Path == nil || route.Method == nil {
			return errors.New(fmt.Sprint("route path and method are required parameters, cannot be empty for route number: ", position+1))
		}

		// Validate the path
		pathErr := ValidatePath(*route.Path, position)
		if pathErr != nil {
			return pathErr
		}

		// Validate methods
		// Method is a required param.
		methodErr := ValidateMethod(*route.Method, position)
		if methodErr != nil {
			return methodErr
		}
	}

	// Validate the trigger
	// The trigger expression in itself is optional
	// However, if the trigger type is Filter, we need an expression
	// string to be passed
	if pipelineBody.Trigger != nil {
		// Cron is not supported for pipelines
		if *pipelineBody.Trigger.Type == rules.Cron {
			return errors.New("cron is not allowed as a trigger for pipelines")
		}

		if *pipelineBody.Trigger.Type == rules.Filter && pipelineBody.Trigger.Expression == "" {
			return errors.New("trigger expression cannot be empty for trigger types: cron and query")
		}

		// If the expression is passed, validate it
		if pipelineBody.Trigger.Expression != "" {
			var err error
			log.Debug(logTag, ":", "Validating the expression based on the trigger")
			switch *pipelineBody.Trigger.Type {
			case rules.Filter:
				err = rules.ValidateTriggerExpression(pipelineBody.Trigger.Expression)
			case rules.Index:
				err = rules.ValidateIndexTriggerExpression(pipelineBody.Trigger.Expression)
			}

			if err != nil {
				return err
			}
		}

		// Validate timeframe
		if pipelineBody.Trigger.TimeFrame != nil && pipelineBody.Trigger.TimeFrame.StartTime == nil {
			return errors.New("start_time must be present in timeframe")
		}
	}

	if pipelineBody.Stages == nil {
		return errors.New("at least one stage must present")
	}

	// Validate the stages
	err := ValidateStages(*pipelineBody.Stages, *pipelineBody.Routes)
	if err != nil {
		return err
	}

	return nil
}

// Parse the pipeline files from the request and return a pipeline body IN
// struct from that.
func (p *Pipelines) parsePipelineFile(req *http.Request) (*ESPipelineDoc, *Error) {
	// Try to parse the form data
	var maxMemAllowedForParse int64 = 32 << 20
	err := req.ParseMultipartForm(maxMemAllowedForParse)
	if err != nil {
		log.Errorln(logTag, "error while parsing form data, ", err)
		return nil, &Error{
			Err: err,
		}
	}

	// Try to find the `pipeline` key since it will be the main file
	pipelineFile, pipelineDetails, err := req.FormFile("pipeline")

	var fileContent []byte
	var fileExtension string

	var validateContext *string = nil

	// If no error was reported, extract the file content and fileExtension
	if err == nil {
		// Close the file
		defer pipelineFile.Close()

		// Use the pipelineDetail to find out the file extension to accordingly parse
		// it.
		// Make sure pipeline filename contains yaml or JSON at the end
		fileExtension = strings.ToLower(filepath.Ext(pipelineDetails.Filename))

		if fileExtension != ".json" && fileExtension != ".yaml" {
			errMsg := "non JSON or YAML file passed"
			log.Warnln(logTag, errMsg)
			code := http.StatusUnprocessableEntity
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}

		// If it's YAML, convert it to JSON, else just read it into JSON
		fileContent = make([]byte, pipelineDetails.Size)
		_, err = pipelineFile.Read(fileContent)

		if err != nil {
			errMsg := fmt.Sprint("something went wrong while reading the file: ", err)
			log.Warnln(logTag, errMsg)
			code := http.StatusBadRequest
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}

	} else {
		// If fails then try to read the pipeline key as a string
		pipelineStr := req.PostFormValue("pipeline")
		if pipelineStr == "" {
			errMsg := fmt.Sprint("error while reading pipeline key, is it a valid file or string?: ", err.Error())
			log.Warnln(logTag, "error while reading pipeline file, ", err)
			code := http.StatusBadRequest
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}

		// Unmarshal the pipeline string into a config in struct
		var pipelineDetailsIn Config
		err = json.Unmarshal([]byte(pipelineStr), &pipelineDetailsIn)
		if err != nil {
			errMsg := fmt.Sprint("error while unmarshalling pipeline into config: ", err)
			log.Warnln(logTag, errMsg)
			code := http.StatusUnprocessableEntity
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}

		// Make sure both content and type are passed
		if pipelineDetailsIn.Content == nil || pipelineDetailsIn.Type == nil {
			errMsg := "content and extension both are required in JSON body"
			log.Warnln(logTag, errMsg)
			code := http.StatusUnprocessableEntity
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}

		// Parse the validate context
		validateContext = pipelineDetailsIn.ValidateContext

		// Populate fileContent and fileExtension
		fileContent = []byte(*pipelineDetailsIn.Content)

		// Add a prefix `.` to the extension
		fileExtension = fmt.Sprintf(".%s", *pipelineDetailsIn.Type)

		// Validate file extension
		if fileExtension != ".json" && fileExtension != ".yaml" {
			errMsg := "non JSON or YAML file passed"
			log.Warnln(logTag, errMsg)
			code := http.StatusUnprocessableEntity
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}
	}

	var pipelineBody ESPipelineDocIn

	// Parse JSON data
	switch fileExtension {
	case ".yaml":
		// If it is YAML, convert the data to JSON
		err := yaml.Unmarshal(fileContent, &pipelineBody)
		if err != nil {
			errMsg := fmt.Sprint("something went wrong while parsing YAML file to ES pipeline struct: ", err)
			log.Warnln(errMsg)
			code := http.StatusBadRequest
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}
	case ".json":
		err := json.Unmarshal(fileContent, &pipelineBody)
		if err != nil {
			errMsg := fmt.Sprint("something went wrong while parsing JSON file to ES Pipeline struct: ", err)
			log.Warnln(logTag, errMsg)
			code := http.StatusBadRequest
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}
	}

	// If validate context is passed then parse it into the pipeline
	// body.
	if validateContext != nil {
		pipelineBody.ValidateContext = validateContext
	}

	// Make sure at least one stage is passed since we will do validation after
	// scriptRefs are replaced
	if pipelineBody.Stages == nil {
		errMsg := fmt.Sprint("stages cannot be empty, should contain at least one stage")
		log.Warnln(errMsg)
		return nil, &Error{
			Err:  errors.New(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// Handle global vars if passed by the user.
	//
	// We will need to extract the vars, save them and remove them
	// from the pipeline content.
	// If the variable with the key already exists, we will have to
	// overwrite the variables
	if pipelineBody.GlobalVars != nil {
		// We need to check if the var already exists or not.
		// If the var exists, we replace it in ES.
		// If the var doesn't exist, we will create a new variable.
		for position, globalVar := range *pipelineBody.GlobalVars {
			varKey := globalVar.Key

			_, location := GetVarAndLocation(*varKey)

			var URL, Method string

			// Get RS Util to access the RS URL
			rsUtil := plugins.RSUtilInstance()
			URL = rsUtil.URL(true)

			reqBody, err := json.Marshal(globalVar)
			if err != nil {
				errMsg := fmt.Sprintf("error while marshalling the global env number: %d", position+1)
				log.Warnln(logTag, errMsg)
				return nil, &Error{
					Err:  errors.New(errMsg),
					Code: http.StatusBadRequest,
				}
			}

			log.Debugln(logTag, fmt.Sprintf("reqbody for global env with key %s: %s", *varKey, pretty.Formatter(string(reqBody))))

			if location == nil {
				// The variable does not exist and we need to create it.
				URL += "/_pipeline/env"
				Method = http.MethodPost
			} else {
				// The variable already exists, we need to replace it
				// with the passed body.
				URL += fmt.Sprint("/_pipeline/env/", *varKey)
				Method = http.MethodPut
			}

			responseBody, _, makeReqErr := util.MakeRequest(URL, Method, reqBody)
			if makeReqErr != nil {
				errMsg := fmt.Sprintf("Updating global vars failed with error %s for global env number: %d", makeReqErr, position+1)
				log.Warnln(logTag, errMsg)
				return nil, &Error{
					Err:  errors.New(errMsg),
					Code: http.StatusBadRequest,
				}
			}

			log.Debug(logTag, fmt.Sprintf("setting variable for %s returned response: %s", *varKey, string(responseBody)))
		}

		// NOTE: No need to remove the variable from the struct
		// explicitly since it will be removed during conversion
		// to ES version.
	}

	// Replace all scriptRef in the pipelineBody with the script content from form data.
	//
	// Iterate over all the stages in the pipeline and check if scriptRef is not nil.
	// If it is not nil, then we need to parse the file passed in the field.
	// We will expect the form key in the scriptRef value.
	// The content parsed will be written to the script field of that stages so it will be
	// overwritten if it is already defined.
	for stageIndex, stage := range *pipelineBody.Stages {
		if stage.ScriptRef == nil || *stage.ScriptRef == "" {
			continue
		}

		// Don't allow both script and scriptRef to be passed at the same time.
		if stage.Script != nil {
			errMsg := fmt.Sprint("`script` and `scriptRef` cannot be passed at the same time for stage: ", stageIndex+1)
			log.Warnln(logTag, errMsg)
			code := http.StatusConflict
			return nil, &Error{
				Err:  errors.New(errMsg),
				Code: code,
			}
		}

		// If it is defined, try to read the content of the file.
		scriptContent, err := parsePipelineRefFile(req, *stage.ScriptRef, stageIndex)
		if err != nil {
			log.Warnln(logTag, "error occurred while reading script ref file: ", err)
			code := http.StatusBadRequest
			return nil, &Error{
				Err:  err,
				Code: code,
			}
		}

		// Update the script field with the read content from scriptRef
		// Assigning the script
		stage.Script = &scriptContent

		// Update the stage in the pipeline body
		(*pipelineBody.Stages)[stageIndex] = stage
	}

	log.Debug(logTag, "read file content is: ", pretty.Formatter(pipelineBody))

	// Convert the pipeline body in to ES pipeline body.
	esPipeline, err := convertPipelineInToES(pipelineBody)
	if err != nil {
		errMsg := fmt.Sprint("something went wrong while converting pipeline body passed to es pipeline doc: ", err)
		log.Errorln(logTag, errMsg)
		return nil, &Error{
			Err: errors.New(errMsg),
		}
	}

	// Add the original config to the body
	fileContentStr := string(fileContent)
	fileExtensionStr := fileExtension[1:]
	esPipeline.OriginalConfig = &Config{
		Content: &fileContentStr,
		Type:    &fileExtensionStr,
	}

	return &esPipeline, nil
}

// Get the analytics usage for the pipeline based on the passed
// filters
func (p *Pipelines) getPipelinesUsage() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// Extract the passed query params from the request query
		queryParams := analytics.RangeQueryParams(req.URL.Query())

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size

		// Extract the valid custom filters from the req query
		filters := analytics.GetCustomFilters(req.URL.Query())

		usageInBytes, err := p.invocationEs.queryPipelinesUsage(req.Context(), queryParams.From, queryParams.To, queryParams.Size, filters)
		if err != nil {
			errMsg := fmt.Sprint("error occurred while extracting pipeline usage data, ", err)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, rw, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(rw, usageInBytes, http.StatusOK)
	}
}

// getPipelineStagesUsage returns the stage aggregation result based
// on the passed ID.
func (p *Pipelines) getPipelineStagesUsage() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		// Extract the passed query params from the request query
		queryParams := analytics.RangeQueryParams(req.URL.Query())

		// Extract the pipeline ID
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size

		// Extract the valid custom filters from the req query
		filters := analytics.GetCustomFilters(req.URL.Query())

		usageInBytes, err := p.invocationEs.queryPipelineStageUsage(req.Context(), queryParams.From, queryParams.To, queryParams.Size, pipelineID, filters)
		if err != nil {
			errMsg := fmt.Sprint("error occurred while extracting pipeline stage usage data, ", err)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, rw, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(rw, usageInBytes, http.StatusOK)
	}
}

// getPipelineVersionTimeTaken returns the avg time taken by versions for the last
// month.
func (p *Pipelines) getPipelineVersionTimeTaken() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the passed query params from the request query
		queryParams := analytics.RangeQueryParams(req.URL.Query())

		// Extract the pipeline ID
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size

		// Extract the valid custom filters from the req query
		filters := analytics.GetCustomFilters(req.URL.Query())

		timeTakenInBytes, timeTakenErr := p.invocationEs.queryPipelineVersionTimeTaken(req.Context(), queryParams.From, queryParams.To, queryParams.Size, pipelineID, filters)
		if timeTakenErr != nil {
			errMsg := fmt.Sprint("error occurred while extracting pipeline avg time taken data: ", timeTakenErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, timeTakenErr.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, timeTakenInBytes, http.StatusOK)
	}
}

// getPipelineStageTimeTaken returns the avg time taken by stages for the last
// month
func (p *Pipelines) getPipelineStageTimeTaken() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the passed query params from the request query
		queryParams := analytics.RangeQueryParams(req.URL.Query())

		// Extract the pipeline ID
		vars := mux.Vars(req)
		pipelineID := vars["id"]
		versionID := vars["version_id"]

		// Convert version into an integer
		versionAsInt, versionIntErr := strconv.Atoi(versionID)
		if versionIntErr != nil {
			errMsg := fmt.Errorf("invalid value passed as version: %s", versionIntErr.Error())
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg.Error(), http.StatusBadRequest)
			return
		}

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size

		// Extract the valid custom filters from the req query
		filters := analytics.GetCustomFilters(req.URL.Query())

		usageInBytes, err := p.invocationEs.queryPipelineVersionStageTimeTaken(req.Context(), queryParams.From, queryParams.To, size, pipelineID, versionAsInt, filters)
		if err != nil {
			errMsg := fmt.Sprint("error occurred while extracting pipeline stage usage data for last month, ", err)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, usageInBytes, http.StatusOK)
	}
}

// getPipelineStageErrorRate returns the error rate by stages for the last
// month
func (p *Pipelines) getPipelineStageErrorRate() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the passed query params from the request query
		queryParams := analytics.RangeQueryParams(req.URL.Query())

		// Extract the pipeline ID
		vars := mux.Vars(req)
		pipelineID := vars["id"]
		versionID := vars["version_id"]

		// Convert version into an integer
		versionAsInt, versionIntErr := strconv.Atoi(versionID)
		if versionIntErr != nil {
			errMsg := fmt.Errorf("invalid value passed as version: %s", versionIntErr.Error())
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg.Error(), http.StatusBadRequest)
			return
		}

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size

		// Extract the valid custom filters from the req query
		filters := analytics.GetCustomFilters(req.URL.Query())

		usageInBytes, err := p.invocationEs.queryPipelineVersionStageErrorRate(req.Context(), queryParams.From, queryParams.To, size, pipelineID, versionAsInt, filters)
		if err != nil {
			errMsg := fmt.Sprint("error occurred while extracting pipeline stage usage data for last month, ", err)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, usageInBytes, http.StatusOK)
	}
}

// getPipelineVersionErrorRate returns the error rate by versions for the last
// month
func (p *Pipelines) getPipelineVersionErrorRate() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the passed query params from the request query
		queryParams := analytics.RangeQueryParams(req.URL.Query())

		// Extract the pipeline ID
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size

		// Extract the valid custom filters from the req query
		filters := analytics.GetCustomFilters(req.URL.Query())

		timeTakenInBytes, timeTakenErr := p.invocationEs.queryPipelineVersionErrorRate(req.Context(), queryParams.From, queryParams.To, queryParams.Size, pipelineID, filters)
		if timeTakenErr != nil {
			errMsg := fmt.Sprint("error occurred while extracting pipeline error rate data: ", timeTakenErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, timeTakenErr.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, timeTakenInBytes, http.StatusOK)
	}
}

// getPipelineVersionUsage returns the version aggregation result based on the
// passed ID.
func (p *Pipelines) getPipelineVersionUsage() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the passed query params from the request query
		queryParams := analytics.RangeQueryParams(req.URL.Query())

		// Extract the pipeline ID
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size

		// Extract the valid custom filters from the req query
		filters := analytics.GetCustomFilters(req.URL.Query())

		usageInBytes, usageFetchErr := p.invocationEs.queryPipelineVersionUsage(req.Context(), queryParams.From, queryParams.To, queryParams.Size, pipelineID, filters)
		if usageFetchErr != nil {
			errMsg := fmt.Sprint("error while fetching pipeline version usage data: ", usageFetchErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, usageInBytes, http.StatusOK)
	}
}

// getPipelinesVars will return all the pipeline vars present in the current
// cluster
func (p *Pipelines) getPipelinesVars() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Get all the pipeline vars present
		pipelineVars := cachedVars

		// Marshal the vars and return them
		varsAsBytes, marshalErr := json.Marshal(pipelineVars)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error occurred while marshalling the pipeline vars: ", marshalErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, varsAsBytes, http.StatusOK)
	}
}

// getPipelineVar will get the pipeline variable using the passed key
func (p *Pipelines) getPipelineVar() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the key from the URL
		vars := mux.Vars(req)
		varKey := vars["key"]

		pipelineVarBody, _ := GetVarAndLocation(varKey)

		if pipelineVarBody == nil {
			errMsg := fmt.Sprintf("%s: no pipeline environment present with this key", varKey)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// If found, return it after marshalling
		varInBytes, marshalErr := json.Marshal(pipelineVarBody)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling the pipeline env body, ", marshalErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, varInBytes, http.StatusOK)
	}
}

// postPipelineVar will create the pipeline variable passed in the database
func (p *Pipelines) postPipelineVar() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the body passed for pipeline var
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			errMsg := fmt.Sprint("error while reading the request body, ", err)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		defer req.Body.Close()

		// Unmarshal the body into pipelineVar
		var pipelineVarBody PipelineVar
		unmarshalErr := json.Unmarshal(reqBody, &pipelineVarBody)

		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling request body into pipeline environment, ", unmarshalErr)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// If it is a local request, just update the cache and return
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update the body locally
			AddVar(pipelineVarBody)

			// Return with a response
			util.WriteBackRaw(w, reqBody, http.StatusOK)
			return
		}

		// Validate the variable
		validateErr := validatePipelineVar(pipelineVarBody, false)
		if validateErr != nil {
			errMsg := fmt.Sprint("error occurred while validating the body, ", validateErr)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Add createdAt, updatedAt and ID
		varId := uuid.New().String()
		createdAt := time.Now().Unix()
		pipelineVarBody.ID = &varId
		pipelineVarBody.CreatedAt = &createdAt
		pipelineVarBody.UpdatedAt = &createdAt

		// Save the var in the index
		saveErr := p.varEs.createVar(req.Context(), varId, pipelineVarBody)
		if saveErr != nil {
			errMsg := fmt.Sprint("error while saving the environment to the index, ", saveErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Invoke ACCAPI
		// We're sending the request body as the final parsed struct to be stored in cache
		// The reason for doing is to avoid recreating the properties like `id` and `createdAt` that
		// can lead to inconsistency.
		//
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Convert the var body to a map
			varMap, err := pipelineVarBody.ToMap()
			if err != nil {
				errMsg := fmt.Sprint("error while converting env to map, ", err)
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPost,
				URL:    "/_pipelines/env",
				Body:   varMap, // forward body
			})

			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered creating environment")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Add the variable to cache
			AddVar(pipelineVarBody)
		}

		// If was saved successfully, marshal the saved body and return it.
		savedBodyInBytes, err := json.Marshal(pipelineVarBody)
		if err != nil {
			errMsg := fmt.Sprint("error while marshalling the response body, ", err)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Write the response body back
		util.WriteBackRaw(w, savedBodyInBytes, http.StatusOK)

	}
}

// deletePipelineVar will delete the pipeline variable with the passed key
// if it is present.
func (p *Pipelines) deletePipelineVar() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the key from the request body
		vars := mux.Vars(req)
		varKey := vars["key"]

		// Make sure that the key is present in the cluster
		pipelineVarDoc, location := GetVarAndLocation(varKey)

		if pipelineVarDoc == nil {
			errMsg := fmt.Sprintf("%s: key is not present in the cluster", varKey)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// If local then remove the var from cache and return
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			RemoveVar(*location)

			util.WriteBackMessage(w, "Environment deleted successfully!", http.StatusOK)
			return
		}

		// Remove the variable from ES
		deleteErr := p.varEs.deleteVar(req.Context(), *pipelineVarDoc.ID, varKey)
		if deleteErr != nil {
			errMsg := fmt.Sprint("error while deleting the environment, ", deleteErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Invoke ACCAPI
		// We're sending the request body as the final parsed struct to be stored in cache
		// The reason for doing is to avoid recreating the properties like `id` and `createdAt` that
		// can lead to inconsistency.
		//
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodDelete,
				URL:    "/_pipelines/env/" + varKey,
			})

			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}

			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered deleting environment")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			RemoveVar(*location)
		}

		util.WriteBackMessage(w, "Environment deleted successfully!", http.StatusOK)
		return
	}
}

// putPipelineVar will update the pipeline variable with the passed key
// and the body.
func (p *Pipelines) putPipelineVar() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the key from the request
		vars := mux.Vars(req)
		varKey := vars["key"]

		// Read the request body
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			errMsg := fmt.Sprint("error while reading the request body, ", err)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		defer req.Body.Close()

		// Unmarshal the body into pipelineVar
		var pipelineVarBody PipelineVar
		unmarshalErr := json.Unmarshal(reqBody, &pipelineVarBody)

		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling request body into pipeline env, ", unmarshalErr)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		pipelineVar, location := GetVarAndLocation(varKey)
		if pipelineVar == nil {
			errMsg := fmt.Sprintf("%s: invalid environment key passed", varKey)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// If it is a local update request, then just update the cache
		// and return without updating ES
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			UpdateVar(*location, pipelineVarBody)

			util.WriteBackMessage(w, "Environment updated sucessfully!", http.StatusOK)
			return
		}

		// Validate the pipeline body passed
		validateErr := validatePipelineVar(pipelineVarBody, true)
		if validateErr != nil {
			errMsg := fmt.Sprint("error while validating the pipeline, ", validateErr)
			log.Warnln(logTag, validateErr)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If the pipeline is validated, set the updatedAt and the ID
		// update the body in ES
		pipelineVarBody.ID = pipelineVar.ID
		pipelineVarBody.CreatedAt = pipelineVar.CreatedAt
		pipelineVarBody.Key = &varKey

		updatedAt := time.Now().Unix()
		pipelineVarBody.UpdatedAt = &updatedAt

		updateErr := p.varEs.updateVar(req.Context(), *pipelineVar.ID, pipelineVarBody)
		if updateErr != nil {
			errMsg := fmt.Sprint("error while updating the pipeline environment in ES, ", updateErr)
			log.Errorln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Invoke ACCAPI
		// If ACCAPI is invoked then it will handle updating the cache in all the
		// nodes present so we do not need to udpate the cached ourselves
		// in order to remove redundancy.
		if util.ShouldProxyToACCAPI() {
			varAsMap, err := pipelineVarBody.ToMap()
			if err != nil {
				errMsg := fmt.Sprint("error while converting passed body to map, ", err)
				log.Errorln(logTag, errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_pipeline/env/" + varKey,
				Body:   varAsMap,
			})

			if err != nil {
				errMsg := fmt.Sprint("error while sending proxy accapi request, ", err)
				log.Errorln(logTag, errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			if res != nil {
				errMsg := fmt.Sprint("error while sending proxy request with response: ", res)
				log.Errorln(logTag, errMsg)

				util.WriteBackMessage(w, "Environment updated sucessfully!", http.StatusOK)
				return
			}
		} else {
			UpdateVar(*location, pipelineVarBody)
		}

		util.WriteBackMessage(w, "Environment updated sucessfully!", http.StatusOK)
	}
}

// getPipelineVersions will get all the pipeline versions for the passed
// pipeline ID.
func (p *Pipelines) getPipelineVersions() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the pipeline ID
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		// Get the pipeline from cache using the passed ID.
		pipelineDoc, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipelineDoc == nil {
			errMsg := fmt.Sprint("pipeline not found with ID: ", pipelineID)
			log.Warnln(logTag, errMsg)

			util.WriteBackMessage(w, errMsg, http.StatusNotFound)
			return
		}

		// NOTE: This is a special case during the pipeline version
		// feature rollout that a pipeline versions may be 0.
		//
		// In this case, return the current pipeline since that is the only
		// version of the pipeline available.
		if pipelineDoc.Versions == nil || len(*pipelineDoc.Versions) == 0 {
			defaultPipelineVersion := make([]Version, 0)

			// Handle marshalling the pipeline
			pipelineMarshalled, pipelineMarshalErr := json.Marshal(pipelineDoc)
			if pipelineMarshalErr != nil {
				errMsg := fmt.Sprint("error while marshalling current pipeline to return as only version with err: ", pipelineMarshalErr)
				log.Errorln(logTag, errMsg)

				util.WriteBackMessage(w, errMsg, http.StatusInternalServerError)
				return
			}
			pipelineAsString := string(pipelineMarshalled)

			pipelineVersion := 1

			defaultPipelineVersion = append(defaultPipelineVersion, Version{Content: &pipelineAsString, Version: &pipelineVersion})
			pipelineDoc.Versions = &defaultPipelineVersion

			pipelineDoc.LiveVersion = &pipelineVersion
		}

		// Fetch the version stats to return them in response
		versionStats, versionStatErr := p.invocationEs.queryPipelineVersionStats(req.Context(), pipelineID)
		if versionStatErr != nil {
			errMsg := fmt.Sprintf("error while fetching version stats from ES: %s", versionStatErr.Error())
			log.Errorln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Handle returning the pipeline versions properly.
		// Go through every version of the pipeline available and return an array of
		// configOut.
		pipelineVersionsOut := make([]ConfigOut, 0)

		for _, version := range *pipelineDoc.Versions {
			var versionAsPipeline ESPipelineDoc
			versionUnmarshalErr := json.Unmarshal([]byte(*version.Content), &versionAsPipeline)

			if versionUnmarshalErr != nil {
				errMsg := fmt.Sprintf("error while unmarshall`ing the pipeline version number: `%d` with error: %s", *version.Version, versionUnmarshalErr)
				log.Warnln(logTag, errMsg)

				util.WriteBackMessage(w, errMsg, http.StatusInternalServerError)
				return
			}

			// Convert the pipeline into a configOut object
			configOutForVersion := getConfigOut(versionAsPipeline, false)
			configOutForVersion.Version = version.Version
			configOutForVersion.VersionDescription = version.Description

			// Check if live version should be set or not
			defaultIsLive := false
			configOutForVersion.IsLive = &defaultIsLive

			if *version.Version == *pipelineDoc.LiveVersion {
				isLive := true
				configOutForVersion.IsLive = &isLive
			}

			// Try to add the version stat by using the version
			versionStat, ok := versionStats[*version.Version]
			if !ok {
				versionStat = map[string]interface{}{}
			}

			configOutForVersion.Usage = versionStat

			pipelineVersionsOut = append(pipelineVersionsOut, configOutForVersion)
		}

		// Marshal the version array and return it.
		pipelineVersionAsByte, pipelineVersionsErr := json.Marshal(pipelineVersionsOut)
		if pipelineVersionsErr != nil {
			errMsg := fmt.Sprint("error while marshaling version array to return with error: ", pipelineVersionsErr)
			log.Errorln(logTag, errMsg)

			util.WriteBackMessage(w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, pipelineVersionAsByte, http.StatusOK)
	}
}

// postPipelineVersion will handle creating a new version for the pipeline
func (p *Pipelines) postPipelineVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		// Find the pipeline with ID
		pipelineWithId, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipelineWithId == nil {
			errMsg := "pipeline with ID not found"
			log.Warnln(logTag, errMsg)

			util.WriteBackMessage(w, errMsg, http.StatusBadRequest)
			return
		}

		// If the pipeline passed is correct, we need to add a version number for it.
		//
		// NOTE: Deleting a pipeline version is not an option, this means the version ID's
		// can be determined easily by just checking the length of the version field for the
		// current pipeline.
		versionToUse := getNextVersion(*pipelineWithId)

		pipelineBody, parseErr := p.parsePipelineFile(req)
		if parseErr != nil {
			code := http.StatusInternalServerError
			if parseErr.Code != 0 {
				code = parseErr.Code
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, parseErr.Err.Error(), code)
			return
		}

		// We need to validate the pipeline to some extent to make sure it's valid.
		validateErr := p.validatePipelineBody(*pipelineBody)
		if validateErr != nil {
			errMsg := fmt.Sprint("invalid pipeline body passed: ", validateErr)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Set the pipeline createdAt
		// Add createdAt
		createdAt := time.Now().Unix()
		pipelineBody.CreatedAt = &createdAt

		pipelineBody.ID = pipelineWithId.ID
		pipelineBody.Enabled = pipelineWithId.Enabled

		// Marshal the pipeline doc JSON
		marshalledVersionPipeline, marshalErr := json.Marshal(pipelineBody)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling passed version of pipeline to store it: ", marshalErr)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)

			return
		}
		marshalledPipelineInString := string(marshalledVersionPipeline)

		// Try to parse the description of the version
		versionDescriptionStr := req.PostFormValue("versionDescription")

		versionInDoc := &Version{
			Version:     &versionToUse,
			Content:     &marshalledPipelineInString,
			Description: &versionDescriptionStr,
		}

		// Update the pipeline upstream as well as cache and handle updating the
		// nodes with the updated pipeline as well.
		*pipelineWithId.Versions = append(*pipelineWithId.Versions, *versionInDoc)

		// Update ES with the updated pipeline
		esErr := p.es.updatePipeline(req.Context(), pipelineID, *pipelineWithId)
		if esErr != nil {
			log.Errorln(logTag, ": error occurred while writing body to ES, ", esErr)
			telemetry.WriteBackErrorWithTelemetry(req, w, esErr.Error(), http.StatusInternalServerError)
			return
		}

		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Convert the ESPipelineDoc to map
			pipelineMap, err := PipelineToMap(*pipelineWithId)
			if err != nil {
				log.Errorln(logTag, ": error while converting body to map, ", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_pipeline/" + pipelineID,
				Body:   pipelineMap,
			})

			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered creating pipeline version")
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		} else {
			// Update Cache
			ok := UpdatePipelineInCache(pipelineID, *pipelineWithId)
			if !ok {
				msg := "Error encountered while updating the pipeline"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
		}

		util.WriteBackMessage(w, fmt.Sprint("Version created successfully with ID: ", versionToUse), http.StatusCreated)
	}
}

// putPipelineVersion will update a pipeline version based on the
// passed pipeline ID and version ID.
func (p *Pipelines) putPipelineVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		pipelineVersionID := vars["version_id"]
		versionAsInt, convertErr := strconv.Atoi(pipelineVersionID)
		if convertErr != nil {
			errMsg := fmt.Sprint("invalid version passed for pipeline: ", convertErr)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Get the pipeline with the pipeline ID passed.
		pipelineBody, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipelineBody == nil {
			errMsg := fmt.Sprint("no pipeline found with ID: ", pipelineID)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// If the version is 1 and the pipeline doesn't have versions from prior,
		// then do not throw an error if the version does not exist.
		versionIndex := new(int)
		if pipelineBody.Versions != nil {
			// If the pipeline is found, try to extract the version with the passed
			// version ID.
			var pipelineVersion *Version
			pipelineVersion, versionIndex = GetPipelineVersion(*pipelineBody, versionAsInt)
			if pipelineVersion == nil {
				errMsg := fmt.Sprintf("pipeline version with version ID: %d not found", versionAsInt)
				log.Warnln(logTag, errMsg)

				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}
		} else {
			defaultVersionIndex := 0
			versionIndex = &defaultVersionIndex

			// Set versions as an array since it is referenced later on.
			emptyVersionArr := make([]Version, 0)
			pipelineBody.Versions = &emptyVersionArr

			// Also set the live version as 1
			defaultLiveVersion := 1
			pipelineBody.LiveVersion = &defaultLiveVersion

			defaultVersionDesc := "v1 pipeline"
			emptyVersionArr = append(emptyVersionArr, Version{Version: &defaultLiveVersion, Description: &defaultVersionDesc})
		}

		// Parse the passed pipeline body and validate it as well.
		parsedPipelineBody, parseErr := p.parsePipelineFile(req)
		if parseErr != nil {
			code := http.StatusInternalServerError
			if parseErr.Code != 0 {
				code = parseErr.Code
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, parseErr.Err.Error(), code)
			return
		}

		parsedPipelineBody.ID = pipelineBody.ID
		parsedPipelineBody.CreatedAt = pipelineBody.CreatedAt

		// We need to validate the pipeline to some extent to make sure it's valid.
		validateErr := p.validatePipelineBody(*parsedPipelineBody)
		if validateErr != nil {
			errMsg := fmt.Sprint("invalid pipeline body passed: ", validateErr)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		updatedAt := time.Now().Unix()
		pipelineBody.UpdatedAt = &updatedAt

		// Set the updated at in the pipeline version as well
		parsedPipelineBody.UpdatedAt = &updatedAt

		// If the body is valid, update the updatedAt date for the pipeline and set the new version
		versionAsBytes, versionMarshalErr := json.Marshal(parsedPipelineBody)
		if versionMarshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling version passed to store it properly: ", versionMarshalErr)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		versionAsStr := string(versionAsBytes)

		(*pipelineBody.Versions)[*versionIndex].Content = &versionAsStr

		// Update the body in ES
		esErr := p.es.updatePipeline(req.Context(), pipelineID, *pipelineBody)
		if esErr != nil {
			log.Errorln(logTag, ": error occurred while writing body to ES, ", esErr)
			telemetry.WriteBackErrorWithTelemetry(req, w, esErr.Error(), http.StatusInternalServerError)
			return
		}

		// Since the pipeline version is updated, update it locally and remotely.
		//
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Convert the ESPipelineDoc to map
			pipelineMap, err := PipelineToMap(*pipelineBody)
			if err != nil {
				log.Errorln(logTag, ": error while converting body to map, ", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_pipeline/" + pipelineID,
				Body:   pipelineMap,
			})

			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered creating pipeline version")
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		} else {
			// Update Cache
			ok := UpdatePipelineInCache(pipelineID, *pipelineBody)
			if !ok {
				msg := "Error encountered while updating the pipeline"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
		}

		util.WriteBackMessage(w, "Pipeline version updated successfully", http.StatusOK)
	}
}

// setLiveVersion will set the live version according to the users
// request
func (p *Pipelines) setLiveVersion() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		pipelineID := vars["id"]

		pipelineVersionID := vars["version_id"]
		versionAsInt, convertErr := strconv.Atoi(pipelineVersionID)
		if convertErr != nil {
			errMsg := fmt.Sprint("invalid version passed for pipeline: ", convertErr)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Get the pipeline with the pipeline ID passed.
		pipelineBody, _ := GetPipelineAndLocFromCache(pipelineID)
		if pipelineBody == nil {
			errMsg := fmt.Sprint("no pipeline found with ID: ", pipelineID)
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		_, versionLocation := GetPipelineVersion(*pipelineBody, versionAsInt)
		if versionLocation == nil {
			errMsg := "passed version with ID not present"
			log.Warnln(logTag, errMsg)

			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusNotFound)
			return
		}

		// Since the version is present, we can set it to live.
		pipelineBody.LiveVersion = &versionAsInt

		updatedAt := time.Now().Unix()
		pipelineBody.UpdatedAt = &updatedAt

		// Update the body in ES
		esErr := p.es.updatePipeline(req.Context(), pipelineID, *pipelineBody)
		if esErr != nil {
			log.Errorln(logTag, ": error occurred while writing body to ES, ", esErr)
			telemetry.WriteBackErrorWithTelemetry(req, w, esErr.Error(), http.StatusInternalServerError)
			return
		}

		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Convert the ESPipelineDoc to map
			pipelineMap, err := PipelineToMap(*pipelineBody)
			if err != nil {
				log.Errorln(logTag, ": error while converting body to map, ", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_pipeline/" + pipelineID,
				Body:   pipelineMap,
			})

			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered creating pipeline version")
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		} else {
			// Update Cache
			ok := UpdatePipelineInCache(pipelineID, *pipelineBody)
			if !ok {
				msg := "Error encountered while updating the pipeline"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
		}

		util.WriteBackMessage(w, "pipeline version set to live successfully", http.StatusOK)
	}
}

// getValidateLogsAndTime will get the logs and time based on the passed
// validateID.
func (p *Pipelines) getValidateLogsAndTime() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		validateId := vars["validateId"]

		if validateId == "" {
			errMsg := fmt.Sprint("`validateId` needs to be valid! Got ", validateId)
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Check if the validate ID exists
		delayedLogs := p.validateSession.GetLogs(validateId)
		if delayedLogs == nil {
			errMsg := fmt.Sprint("passed validateId is invalid!")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Now that sessionId is present, access the response
		//
		// Make sure that the response resolved only after waiting 60 secs since
		// the response might still be loading.
		timerStart := time.Now()

		log.Debug(logTag, ": waiting for response to be ready or timeout")
		for time.Since(timerStart).Seconds() <= 60 && !delayedLogs.GetIsReady() {
			continue
		}

		// Seems like response is ready
		logs := delayedLogs.GetLogs()
		timeTaken := delayedLogs.GetTimeTaken()

		// Handle scenario of logs being nil
		if logs == nil {
			logs = &StageLogTracker{}
		}

		responseToReturn := map[string]interface{}{
			"console_logs": MergeStageLogs(logs.value),
			"time_taken":   timeTaken,
		}

		bodyInBytes, marshalErr := json.Marshal(responseToReturn)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshaling response body: ", marshalErr.Error())
			log.Errorln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, bodyInBytes, http.StatusOK)
	}
}
