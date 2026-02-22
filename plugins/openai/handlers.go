package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/buger/jsonparser"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

// buildResponseBody will build the response body to return
// for a fetch AI answer call
func buildResponseBody(responseDetails *InternalChatGPTResponse) ([]byte, int, error) {
	// If the response has failed, return the response as is
	if responseDetails.GetIsFailed() {
		// TODO: Check whether or not request is present and accordingly return
		// request as well?
		return responseDetails.Response(), http.StatusBadRequest, nil
	}

	// At this point either it timed out or we have the response ready
	if !responseDetails.GetIsReady() {
		// It timed out and the response is not ready yet
		errMsg := "request timed out while waiting for the ChatGPT response to be fetched"
		return nil, http.StatusGatewayTimeout, fmt.Errorf(errMsg)
	}

	// Inject the request body as well in the response
	requestToReturn := responseDetails.Request()

	// Make sure struct is ready for usage
	lastQuestion := ""
	if requestToReturn.IsStructReady() {
		requestStruct := requestToReturn.Struct()

		if len(requestStruct.Messages) > 0 {
			lastQuestion = requestStruct.Messages[len(requestStruct.Messages)-1].Content
		}
	}

	responseToReturn := map[string]interface{}{
		"question": lastQuestion,
	}

	// Extract the response choice
	responseText, extractErr := jsonparser.GetString(responseDetails.Response(), "choices", "[0]", "message", "content")
	if extractErr != nil {
		// Cannot continue, throw the error
		errMsg := fmt.Sprint("error while extracting response from ChatGPT response: ", extractErr.Error())
		return nil, http.StatusInternalServerError, fmt.Errorf(errMsg)
	}

	// Extract the model
	modelAsText, modelExtractErr := jsonparser.GetString(responseDetails.Response(), "model")
	if modelExtractErr != nil {
		errMsg := fmt.Sprint("error while extracting model: ", modelExtractErr.Error())
		return nil, http.StatusInternalServerError, fmt.Errorf(errMsg)
	}

	answerMap := map[string]interface{}{
		"text":  responseText,
		"model": modelAsText,
	}

	// Try to extract the documentIds as well
	documentIdsAsBytes, documentIdsType, _, documentIdsExtractErr := jsonparser.Get(responseDetails.Response(), "documentIds")
	if documentIdsExtractErr != nil || documentIdsType != jsonparser.Array {
		errMsg := fmt.Sprint("error while extracting documentIds, either error occurred or value is not of type array. Err is: `", documentIdsExtractErr, "` and type is: `", documentIdsType, "`")
		log.Warnln(logTag, ": ", errMsg)
	} else {
		// Set the documentId's
		documentIdsArr := make([]string, 0)
		unmarshalErr := json.Unmarshal(documentIdsAsBytes, &documentIdsArr)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling documentIds into array: ", unmarshalErr.Error())
			return nil, http.StatusInternalServerError, fmt.Errorf(errMsg)
		}

		answerMap["documentIds"] = documentIdsArr
	}

	responseToReturn["answer"] = answerMap

	// Marshal the body and return bytes
	bodyWithResponse, marshalErr := json.Marshal(responseToReturn)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling body to return: ", marshalErr.Error())
		return nil, http.StatusInternalServerError, fmt.Errorf(errMsg)
	}

	return bodyWithResponse, http.StatusOK, nil
}

// fetchAIAnswer will take care of returning the proper ChatGPT
// response based on the passed sessionId
func (r *OpenAI) fetchAIAnswer() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the sessionId from the path param.
		vars := mux.Vars(req)
		sessionId := vars["AISessionId"]

		if sessionId == "" {
			errMsg := "`AISessionId` is required"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Check if the sessionId is valid
		responseDetails := r.sessionMap.GetResponse(sessionId)
		if responseDetails == nil {
			// This means the sessionId was never added or has been deleted
			errMsg := fmt.Sprintf("%s: invalid sessionId passed, does not exist!", sessionId)
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
		for time.Since(timerStart).Seconds() <= 60 && !responseDetails.GetIsReady() {
			continue
		}

		bodyWithResponse, statusCode, fetchErr := buildResponseBody(responseDetails)
		if fetchErr != nil {
			log.Warnln(logTag, ": ", fetchErr.Error())
			telemetry.WriteBackErrorWithTelemetry(req, w, fetchErr.Error(), statusCode)
			return
		}

		// Request is ready, return it as is since it's just bytes
		util.WriteBackRaw(w, bodyWithResponse, statusCode)
	}
}

// postOpenAIConfig will set the passed openAI Config details
// and update the cache
func (r *OpenAI) postOpenAIConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Get saved preferences
			response, err := r.es.getSettings(req.Context())
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusNotFound)
				return
			}
			// Update local variable
			r.SetConfig(response)
			util.WriteBackMessage(w, "OpenAI settings saved successfully", http.StatusOK)
			return
		}

		// Extract the body passed for pipeline var
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			errMsg := fmt.Sprint("error while reading the request body, ", err)
			log.Warnln(logTag, errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		defer req.Body.Close()

		var configPassed ConfigDetails
		unmarshalErr := json.Unmarshal(reqBody, &configPassed)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling the passed body into config: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		validatedConfig, validateErr := ValidateConfigPassed(configPassed)
		if validateErr != nil {
			errMsg := fmt.Sprint("error while validating the passed body: ", validateErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		bodyToSave := validatedConfig.ToInternalConfig()

		// Make a simple ping call to ChatGPT with the passed details
		// and accordingly ensure that it is working fine.
		//
		// Validation will happen only if the `enable` is set to `true`
		if bodyToSave.Enable != nil && *bodyToSave.Enable {
			maxTokens := bodyToSave.MaxTokens
			azureUrlToUse := ""
			azureVersionToUse := ""

			if bodyToSave.AzureBaseURL != nil {
				azureUrlToUse = *bodyToSave.AzureBaseURL
			}

			if bodyToSave.AzureVersion != nil {
				azureVersionToUse = *bodyToSave.AzureVersion
			}

			configValidationErr := PingChatGPTWithDetails(
				*bodyToSave.Model,
				*bodyToSave.OpenAIKey,
				maxTokens,
				nil,
				*bodyToSave.APIType,
				azureUrlToUse,
				azureVersionToUse,
				*bodyToSave.SystemPrompt,
			)

			// If the validation failed, throw an error accordingly
			if configValidationErr != nil {
				errMsg := fmt.Sprint("error while validating the passed credentials: ", configValidationErr.Message)
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}
		}

		// Save the validated config in the db now.
		saveErr := r.es.saveSettings(bodyToSave, req.Context())
		if saveErr != nil {
			errMsg := fmt.Sprint("error while saving passed body to index: ", saveErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Invoke ACCAPI
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPost,
				URL:    "/_ai/preferences",
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating openAI preferences")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update cache
			r.SetConfig(bodyToSave)
		}

		util.WriteBackMessage(w, "OpenAI preferences updated successfully", http.StatusOK)
	}
}

// getOpenAIConfig will return the saved OpenAI config details
func (r *OpenAI) getOpenAIConfig() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Get the config in the external structure
		configInExternal := r.GetConfig().ToExternalConfig()
		configInBytes, marshalErr := json.Marshal(configInExternal)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling config into bytes: ", marshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, configInBytes, http.StatusOK)
	}
}

// putSessionAnalytics will accept session analytics and store
// it accordingly
func (r *OpenAI) putSessionAnalytics() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the sessionId from the path param.
		vars := mux.Vars(req)
		sessionId := vars["AISessionId"]

		if sessionId == "" {
			errMsg := "`AISessionId` is required"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Fetch the session details
		responseDetails := r.sessionMap.GetResponse(sessionId)
		if responseDetails == nil {
			errMsg := fmt.Sprintf("%s: invalid sessionId passed, does not exist!", sessionId)
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Parse the request body
		bodyAsBytes, readErr := ioutil.ReadAll(req.Body)
		if readErr != nil {
			errMsg := fmt.Sprint("error while reading body to extract analytics: ", readErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		var analyticsDetails UsefulAnalytics
		unmarshalErr := json.Unmarshal(bodyAsBytes, &analyticsDetails)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling passed body into useful analytics structure: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// No need to validate the request body since it will be validated
		// by the following function

		setErr := r.sessionMap.SetUseful(sessionId, analyticsDetails)
		if setErr != nil {
			errMsg := fmt.Sprint("error while setting useful analytics: ", setErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackMessage(w, "Analytics saved successfully!", http.StatusOK)
	}
}

// getSessionDetails will return the session analytics details for
// the passed sessionId
func (r *OpenAI) getSessionDetails() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the sessionId from the path param.
		vars := mux.Vars(req)
		sessionId := vars["AISessionId"]

		if sessionId == "" {
			errMsg := "`AISessionId` is required"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Fetch the session details
		responseDetails := r.sessionMap.GetResponse(sessionId)
		if responseDetails == nil {
			errMsg := fmt.Sprintf("%s: invalid sessionId passed, does not exist!", sessionId)
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		sessionDetails := responseDetails.GetSession()

		// Remove the internal_response key
		sessionDetails.InternalResponse = nil

		sessionInBytes, marshalErr := json.Marshal(sessionDetails)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling the session details into bytes: ", marshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, sessionInBytes, http.StatusOK)
	}
}

// getSessionAnalytics will return the session analytics for the
// passed duration
func (r *OpenAI) getSessionAnalytics() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the params first
		from, to, size := parseRangeParams(req.URL.Query())

		analyticsResponse, fetchErr := r.analyticsEs.getAISessionAnalytics(req.Context(), from, to, size)
		if fetchErr != nil {
			errMsg := fmt.Sprint("error while fetching analytics ", fetchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, analyticsResponse, http.StatusOK)
	}
}

// getSessionAnalyticsByFilter will return the session analytics based
// on the passed filters
func (r *OpenAI) getSessionAnalyticsByFilter() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Parse the query params
		queryParams := parseFilterParams(req.URL.Query())

		sessionDocs, fetchErr := r.analyticsEs.filterAISessionAnalytics(req.Context(), queryParams)
		if fetchErr != nil {
			errMsg := fmt.Sprint("error while filtering session docs: ", fetchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		responseStructureToReturn := map[string]interface{}{
			"hits":  sessionDocs,
			"count": len(sessionDocs),
		}

		// Marshal the response and return it accordingly.
		docsInBytes, marshalErr := json.Marshal(responseStructureToReturn)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling session docs into bytes: ", marshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, docsInBytes, http.StatusOK)
	}
}

// postFollowUpQuestion will take care of adding a follow-up question
// to ChatGPT
func (r *OpenAI) postFollowUpQuestion() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the sessionId from the path param.
		vars := mux.Vars(req)
		sessionId := vars["AISessionId"]

		if sessionId == "" {
			errMsg := "`AISessionId` is required"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		isSessionExpired := false

		// Check if the sessionId is valid
		responseDetails := r.sessionMap.GetResponse(sessionId)
		if responseDetails == nil {
			// This means the sessionId was never added or has been deleted
			isSessionExpired = true
			errMsg := fmt.Sprintf("%s: invalid sessionId passed, does not exist!", sessionId)
			log.Warnln(logTag, ": ", errMsg)
		}

		// Parse the user passed question from the request body
		bodyAsBytes, readErr := ioutil.ReadAll(req.Body)
		if readErr != nil {
			errMsg := fmt.Sprint("error while reading body to extract follow-up question: ", readErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		var followUpPassed FollowUpRequest
		unmarshalErr := json.Unmarshal(bodyAsBytes, &followUpPassed)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling the body to extract question: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		if followUpPassed.Question == nil || strings.TrimSpace(*followUpPassed.Question) == "" {
			errMsg := "`question` is a required key for follow-up question"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If sessionId is expired (which means we don't have history) and the request
		// body is not passed in the request, we cannot continue
		if isSessionExpired && followUpPassed.Request == nil {
			errMsg := "`request` is required since the sessionId is invalid"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Build the follow-up request body
		followUpReqBody, buildErr := BuildFollowUpBody(responseDetails, followUpPassed)
		if buildErr != nil {
			log.Warnln(logTag, ": ", buildErr.Error())
			telemetry.WriteBackErrorWithTelemetry(req, w, buildErr.Error(), http.StatusBadRequest)
			return
		}

		// If the follow-up body doesn't contain the `model` field, we will
		// add the default value for model.
		if followUpReqBody.Model == "" {
			followUpReqBody.Model = r.GetConfig().GetModel()
		}

		// Since we are not expecting the userId here, we will have to use the
		// user IP address
		userId := req.RemoteAddr

		// If the session is invalid, we will have to register a new session with the
		// passed sessionId
		if responseDetails == nil {
			r.sessionMap.RegisterResponse(sessionId, userId, make([]string, 0), make([]string, 0))
			responseDetails = r.sessionMap.GetResponse(sessionId)
		}

		// Marshal the messages array into a map[string]interface{}
		messagesAsBytes, marshalErr := json.Marshal(followUpReqBody.Messages)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling built messages into bytes to pass to ChatGPT: ", marshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, marshalErr.Error(), http.StatusInternalServerError)
			return
		}

		var messagesAsMap []map[string]interface{}
		json.Unmarshal(messagesAsBytes, &messagesAsMap)

		// Initiate the ChatGPT request
		responseDetails.SetIsReady(false)

		// Make the call to init the request
		err := FetchChatGPTForSession(
			sessionId,
			r.sessionMap,
			followUpReqBody.Model,
			messagesAsMap,
			followUpReqBody.DocumentIds,
			r.GetConfig().Key(),
			followUpReqBody.MaxTokens,
			followUpReqBody.Temperature,
			r.GetConfig().GetAPIType(),
			r.GetConfig().GetAzureURL(),
			r.GetConfig().GetAzureVersion(),
			true,
		)

		// Log the error, if any
		if err != nil {
			errMsg := fmt.Sprint("error while fetching follow-up response from ChatGPT: ", err.Error())
			log.Warnln(logTag, ": ", errMsg)

			// If the err is a valid JSON, return it directly instead of parsing it into
			// the error body.
			if IsValidJSON(err.Error()) {
				util.WriteBackRaw(w, []byte(err.Error()), http.StatusInternalServerError)
			} else {
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			}

			return
		}

		// Build the response
		responseToReturn, statusCode, buildErr := buildResponseBody(responseDetails)
		if buildErr != nil {
			log.Warnln(logTag, ": ", buildErr.Error())
			telemetry.WriteBackErrorWithTelemetry(req, w, buildErr.Error(), statusCode)
			return
		}

		// Request is ready, return it as is since it's just bytes
		util.WriteBackRaw(w, responseToReturn, statusCode)
	}
}

func (r *OpenAI) findFollowUpAnswer(responseDetails *InternalChatGPTResponse, followUpPassed FollowUpRequest, sessionId string, userId string) ([]byte, error, int) {
	// Build the follow-up request body
	followUpReqBody, buildErr := BuildFollowUpBody(responseDetails, followUpPassed)
	if buildErr != nil {
		return nil, buildErr, http.StatusBadRequest
	}

	// If the session is invalid, we will have to register a new session with the
	// passed sessionId
	if responseDetails == nil {
		r.sessionMap.RegisterResponse(sessionId, userId, make([]string, 0), make([]string, 0))
		responseDetails = r.sessionMap.GetResponse(sessionId)
	}

	// Marshal the messages array into a map[string]interface{}
	messagesAsBytes, marshalErr := json.Marshal(followUpReqBody.Messages)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while marshalling built messages into bytes to pass to ChatGPT: ", marshalErr.Error())
		return nil, fmt.Errorf(errMsg), http.StatusInternalServerError
	}

	var messagesAsMap []map[string]interface{}
	json.Unmarshal(messagesAsBytes, &messagesAsMap)

	// Initiate the ChatGPT request
	responseDetails.SetIsReady(false)

	// Make the call to init the request
	err := FetchChatGPTForSessionWithStream(
		sessionId,
		r.sessionMap,
		followUpReqBody.Model,
		messagesAsMap,
		followUpReqBody.DocumentIds,
		r.GetConfig().Key(),
		followUpReqBody.MaxTokens,
		followUpReqBody.Temperature,
		r.GetConfig().GetAPIType(),
		r.GetConfig().GetAzureURL(),
		r.GetConfig().GetAzureVersion(),
		true,
	)

	// Log the error, if any
	if err != nil {
		// If it's a JSON error, we should return it without modifying it.
		if IsValidJSON(err.Error()) {
			return nil, err, http.StatusInternalServerError
		}

		errMsg := fmt.Sprint("error while fetching follow-up response from ChatGPT: ", err.Error())
		return nil, fmt.Errorf(errMsg), http.StatusInternalServerError
	}

	// Inject the response inside the `response` key.
	updatedResponse, setErr := jsonparser.Set([]byte("{}"), responseDetails.Response(), "response")
	if setErr != nil {
		errMsg := fmt.Sprint("error while setting the ChatGPT response inside the `response` key: ", setErr.Error())
		return nil, fmt.Errorf(errMsg), http.StatusInternalServerError
	}

	return updatedResponse, nil, http.StatusOK
}

// postFollowUpQuestionSSE will take care of adding a follow-up
// question and returning the response using a text/event-stream
// content-type
func (r *OpenAI) postFollowUpQuestionSSE() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the sessionId from the path param.
		vars := mux.Vars(req)
		sessionId := vars["AISessionId"]

		sseCallReceivedAt := time.Now().Unix()

		if sessionId == "" {
			errMsg := "`AISessionId` is required"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		isSessionExpired := false

		// Check if the sessionId is valid
		responseDetails := r.sessionMap.GetResponse(sessionId)
		if responseDetails == nil {
			// This means the sessionId was never added or has been deleted
			isSessionExpired = true
			errMsg := fmt.Sprintf("%s: invalid sessionId passed, does not exist!", sessionId)
			log.Warnln(logTag, ": ", errMsg)
		}

		// If the response is already in progress, we will throw an error
		// indicating that we cannot go ahead with a stream.
		if !responseDetails.GetIsReady() || responseDetails.GetIsStreaming() {
			errMsg := fmt.Sprint("A question is already in progress, please try again later!")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Parse the user passed question from the request body
		bodyAsBytes, readErr := ioutil.ReadAll(req.Body)
		if readErr != nil {
			errMsg := fmt.Sprint("error while reading body to extract follow-up question: ", readErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		var followUpPassed FollowUpRequest
		unmarshalErr := json.Unmarshal(bodyAsBytes, &followUpPassed)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling the body to extract question: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		if followUpPassed.Question == nil || strings.TrimSpace(*followUpPassed.Question) == "" {
			errMsg := "`question` is a required key for follow-up question"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If sessionId is expired (which means we don't have history) and the request
		// body is not passed in the request, we cannot continue
		if isSessionExpired && followUpPassed.Request == nil {
			errMsg := "`request` is required since the sessionId is invalid"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Since we are not expecting the userId here, we will have to use the
		// user IP address
		userId := req.RemoteAddr
		errChannel := make(chan error)

		go func(responseDetails *InternalChatGPTResponse, followUpPassed FollowUpRequest, sessionId string, userId string, errorChan chan<- error) {
			// Start the streaming from OpenAI. This will let use use the channel to read the response as it
			// gets streamed
			_, err, _ := r.findFollowUpAnswer(responseDetails, followUpPassed, sessionId, userId)
			errorChan <- err
		}(responseDetails, followUpPassed, sessionId, userId, errChannel)

		// Wait till OpenAI streaming starts
		timerStart := time.Now()

		log.Debug(logTag, ": waiting for streaming to be ready or timeout")
		for time.Since(timerStart).Seconds() <= 60 && !responseDetails.GetIsStreaming() {
			log.Debug(logTag, "streaming: ", responseDetails.GetIsStreaming())
			continue
		}

		// If streaming is still not ready, throw an error here
		if !responseDetails.GetIsStreaming() {
			errMsg := "streaming is not initialized yet, cannot continue, request has timed out!"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusRequestTimeout)
		}

		// Check if the response failed and in such a case return the error
		followUpErr, ok := <-errChannel
		if ok && followUpErr != nil {
			log.Warnln(logTag, ": Streaming failed, returning JSON response with error")
			util.WriteBackRaw(w, []byte(followUpErr.Error()), http.StatusInternalServerError)
			return
		}

		// Fetch the flusher interface from the writer
		flusher, flusherOk := w.(http.Flusher)
		if !flusherOk {
			errMsg := "error while converting writer to flusher interface: could not convert!"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Set the headers for SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		// Fetch the channel from where we will read the responses
		responseChannel := sessionSingleton.GetChannel(sessionId)

		// Capture the time when streaming starts
		streamingStartsAt := time.Now().Unix()

		// Start an infinite loop that ends when the chan resolves
		continueExecution := true
		for continueExecution {
			responseInBytes := <-*responseChannel

			// Remove the `data: ` part
			responseInBytes = bytes.Replace(responseInBytes, []byte("data: "), []byte(""), -1)

			// Split based on 2 newlines
			responseInStr := string(responseInBytes)
			splittedResponse := strings.Split(responseInStr, "\n\n")

			for position, responseEach := range splittedResponse {
				if position == len(splittedResponse)-1 {
					// Last one, so probably empty, skip it!
					continue
				}

				// Check for `[DONE]` to break.
				if strings.Contains(string(responseEach), "[DONE]") {
					log.Debug(logTag, ": exiting loop since done!")
					fmt.Fprintf(w, "data: [DONE]\n\n")
					flusher.Flush()
					continueExecution = false
					break
				}

				dataToWrite, extractErr := jsonparser.GetString([]byte(responseEach), "choices", "[0]", "delta", "content")
				if extractErr != nil {
					log.Warnln(logTag, ": extract error for data: `", string(dataToWrite), "` with err: ", extractErr.Error())
					continue
				}

				fmt.Fprintf(w, "data: %s\n\n", dataToWrite)
				flusher.Flush()
			}
		}

		sessionSingleton.SetSSECallAt(sessionId, sseCallReceivedAt, streamingStartsAt, 0)

		log.Debug(logTag, ": loop ended!")
	}
}

// getResponseWithSSE will return the first response with SSE
func (r *OpenAI) getResponseWithSSE() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the sessionId from the path param.
		vars := mux.Vars(req)
		sessionId := vars["AISessionId"]

		// Capture the SSE call incoming time
		sseCallReceivedAt := time.Now().Unix()

		if sessionId == "" {
			errMsg := "`AISessionId` is required"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Check if the sessionId is valid
		responseDetails := r.sessionMap.GetResponse(sessionId)
		if responseDetails == nil {
			errMsg := fmt.Sprintf("%s: invalid sessionId passed, does not exist!", sessionId)
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Check if the session is not streaming
		if !responseDetails.GetIsStreaming() && !responseDetails.GetIsReady() {
			// Seems like the ChatGPT request was made without streaming enabled!
			errMsg := fmt.Sprint("Seems like internal request to ChatGPT was made without streaming enabled. Please use the `GET /_ai/{AISessionId}` endpoint instead!")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If it is ready, we can directly return the response as JSON
		if responseDetails.GetIsReady() {
			bodyWithResponse, statusCode, fetchErr := buildResponseBody(responseDetails)
			if fetchErr != nil {
				log.Warnln(logTag, ": ", fetchErr.Error())
				telemetry.WriteBackErrorWithTelemetry(req, w, fetchErr.Error(), statusCode)
				return
			}

			// Request is ready, return it as is since it's just bytes
			util.WriteBackRaw(w, bodyWithResponse, statusCode)
			return
		}

		// Seems like streaming is going on, we can open a event-stream
		// and start streaming the response as well.

		// Check if response writer supports flushing
		flusher, flusherOk := w.(http.Flusher)
		if !flusherOk {
			errMsg := fmt.Sprint("Streaming is not supported by the server")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Set the headers for SSE response
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)
		flusher.Flush()

		// Fetch the channel from where we will read the responses
		responseChannel := sessionSingleton.GetChannel(sessionId)

		// Capture the time when streaming starts
		streamingStartsAt := time.Now().Unix()
		var streamingFirstByteWritten int64 = 0

		// Start an infinite loop that ends when the chan resolves
		continueExecution := true
		for continueExecution {
			responseInBytes := <-*responseChannel

			if streamingFirstByteWritten == 0 {
				streamingFirstByteWritten = time.Now().Unix()
			}

			// Remove the `data: ` part
			responseInBytes = bytes.Replace(responseInBytes, []byte("data: "), []byte(""), -1)

			// Split based on 2 newlines
			responseInStr := string(responseInBytes)
			splittedResponse := strings.Split(responseInStr, "\n\n")

			for position, responseEach := range splittedResponse {
				if position == len(splittedResponse)-1 {
					// Last one, so probably empty, skip it!
					continue
				}

				// Check for `[DONE]` to break.
				if strings.Contains(string(responseEach), "[DONE]") {
					log.Debug(logTag, ": exiting loop since done!")
					fmt.Fprintf(w, "data: [DONE]\n\n")
					flusher.Flush()
					continueExecution = false
					break
				}

				dataToWrite, extractErr := jsonparser.GetString([]byte(responseEach), "choices", "[0]", "delta", "content")
				if extractErr != nil {
					log.Warnln(logTag, ": extract error for data: `", string(dataToWrite), "` with err: ", extractErr.Error())
					continue
				}

				fmt.Fprintf(w, "data: %s\n\n", dataToWrite)
				flusher.Flush()
			}
		}

		// Set the SSE call resolved time
		sessionSingleton.SetSSECallAt(sessionId, sseCallReceivedAt, streamingStartsAt, streamingFirstByteWritten)

		log.Debug(logTag, ": loop ended!")
	}
}

// createOrUpdateFAQ will take care of creating the passed FAQ
// or updating it if it already exists.
func (r *OpenAI) createOrUpdateFAQ() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		docId := vars["id"]

		// Make sure ID is not empty
		if docId == "" {
			errMsg := fmt.Sprint("`id` is a required param")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Parse the body and inject the ID
		bodyAsBytes, readErr := ioutil.ReadAll(req.Body)
		if readErr != nil {
			errMsg := fmt.Sprint("error while reading passed body: ", readErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		var FAQBodyPassed FAQBody
		unmarshalErr := json.Unmarshal(bodyAsBytes, &FAQBodyPassed)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling body in FAQBody: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If it is local update request, ES is already updated by the proxy.
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			util.WriteBackMessage(w, "FAQ added successfully!", http.StatusOK)
			return
		}

		// Validate the passed body
		validateErr := ValidateFAQBody(FAQBodyPassed)
		if validateErr != nil {
			errMsg := fmt.Sprint("error while validating passed body: ", validateErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If order is not passed, we will have to inject the order
		if FAQBodyPassed.Order == nil {
			nextFAQOrder := r.getFAQNextOrder(req.Context())
			FAQBodyPassed.Order = &nextFAQOrder
		}

		// Inject the doc extracted from the URL
		FAQBodyPassed.ID = &docId

		// Inject the updated at value
		updatedAt := time.Now().Unix()
		FAQBodyPassed.UpdatedAt = &updatedAt

		// Store the doc in Zinc
		createErr := r.faqEs.createFAQ(req.Context(), FAQBodyPassed)
		if createErr != nil {
			errMsg := fmt.Sprint("error while creating the doc in ES: ", createErr.Error())
			log.Warnln(logTag, ": ", errMsg)
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
			// Send the request to update on other nodes

			// Convert the request body into map[string]interface{}
			reqBody := make(map[string]interface{})
			bodyAsBytes, marshalErr := json.Marshal(FAQBodyPassed)
			if marshalErr != nil {
				errMsg := fmt.Sprint("error while marshalling FAQ body back into bytes: ", marshalErr.Error())
				log.Errorln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			unmarshalErr := json.Unmarshal(bodyAsBytes, &reqBody)
			if unmarshalErr != nil {
				errMsg := fmt.Sprint("error while unmarshalling the FAQ body into a map: ", unmarshalErr.Error())
				log.Errorln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    fmt.Sprintf("/_ai/faq/%s", *FAQBodyPassed.ID),
				Body:   reqBody, // forward body
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered creating FAQ")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		}

		// Return the FAQ body in response
		faqInBytes, marshalErr := json.Marshal(FAQBodyPassed)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling passed FAQ body: ", marshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, faqInBytes, http.StatusOK)
	}
}

// deleteFAQ will delete the FAQ using the passed ID
func (r *OpenAI) deleteFAQ() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		docId := vars["id"]

		// Make sure ID is not empty
		if docId == "" {
			errMsg := fmt.Sprint("`id` is a required param")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If it is local delete request, ES is already updated by the proxy.
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			util.WriteBackMessage(w, "FAQ deleted successfully!", http.StatusOK)
			return
		}

		// Delete from ES
		deleteFromES := r.faqEs.deleteFAQ(req.Context(), docId)
		if deleteFromES != nil {
			// If the error message contains `Error 404`, we will return a
			// 404 status code instead.
			statusCodeToReturn := http.StatusInternalServerError
			errMsg := fmt.Sprint("error while deleting doc from ES: ", deleteFromES.Error())
			if strings.Contains(deleteFromES.Error(), "Error 404") {
				statusCodeToReturn = http.StatusNotFound
				errMsg = "No FAQ found with ID"
			}

			log.Errorln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, statusCodeToReturn)
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
			// Send the request to delete on other nodes

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodDelete,
				URL:    fmt.Sprintf("/_ai/faq/%s", docId),
				Body:   nil,
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered deleting FAQ")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		}

		util.WriteBackMessage(w, "FAQ deleted successfully!", http.StatusOK)
	}
}

// getFAQById will get the FAQ by using the ID passed in the URL
// and return it accordingly
func (r *OpenAI) getFAQById() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		docId := vars["id"]

		// Make sure ID is not empty
		if docId == "" {
			errMsg := fmt.Sprint("`id` is a required param")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Get the FAQ from zinc
		faqInBytes, fetchErr := r.faqEs.getFAQ(req.Context(), docId)
		if fetchErr != nil {
			errMsg := fmt.Sprint("error while getting FAQ item from ES: ", fetchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, faqInBytes, http.StatusOK)
	}
}

// getFAQs will get all the FAQ's from Zinc and return them in sorted
// manner
func (r *OpenAI) getFAQs() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Parse the `from` and `size` values
		urlValues := req.URL.Query()

		fromAsInt := 0
		sizeAsInt := 10

		fromValue := urlValues.Get("from")
		if fromValue != "" {
			fromToInt, parseErr := strconv.Atoi(fromValue)
			if parseErr != nil {
				errMsg := fmt.Sprint("invalid value passed for `from`, should be an integer: ", parseErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}

			fromAsInt = fromToInt
		}

		sizeValue := urlValues.Get("size")
		if sizeValue != "" {
			sizeToInt, sizeParseErr := strconv.Atoi(sizeValue)
			if sizeParseErr != nil {
				errMsg := fmt.Sprint("invalid value passed for `size`, should be an integer: ", sizeParseErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}

			sizeAsInt = sizeToInt
		}

		searchResults, searchErr := r.faqEs.getFAQs(req.Context(), fromAsInt, sizeAsInt)
		if searchErr != nil {
			errMsg := fmt.Sprint("error while searching for the FAQ's: ", searchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, searchResults, http.StatusOK)
	}
}

// getFAQsBySearchBox will get all the FAQ's from Zinc and return them in sorted
// manner
func (r *OpenAI) getFAQsBySearchBox() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		searchBoxId := vars["searchboxId"]
		// Parse the `from` and `size` values
		urlValues := req.URL.Query()

		fromAsInt := 0
		sizeAsInt := 10

		fromValue := urlValues.Get("from")
		if fromValue != "" {
			fromToInt, parseErr := strconv.Atoi(fromValue)
			if parseErr != nil {
				errMsg := fmt.Sprint("invalid value passed for `from`, should be an integer: ", parseErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}

			fromAsInt = fromToInt
		}

		sizeValue := urlValues.Get("size")
		if sizeValue != "" {
			sizeToInt, sizeParseErr := strconv.Atoi(sizeValue)
			if sizeParseErr != nil {
				errMsg := fmt.Sprint("invalid value passed for `size`, should be an integer: ", sizeParseErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
				return
			}

			sizeAsInt = sizeToInt
		}

		searchResults, searchErr := r.faqEs.getFAQsBySearchBox(req.Context(), searchBoxId, fromAsInt, sizeAsInt)
		if searchErr != nil {
			errMsg := fmt.Sprint("error while searching for the FAQs by searchBoxId: ", searchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, searchResults, http.StatusOK)
	}
}

// createSession will create a new ChatGPT session based on the input question
// by the user.
func (r *OpenAI) createSession() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Extract the request body passed by the user.
		// Parse the user passed question from the request body
		bodyAsBytes, readErr := ioutil.ReadAll(req.Body)
		if readErr != nil {
			errMsg := fmt.Sprint("error while reading body for session: ", readErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		var sessionContext SessionContext
		unmarshalErr := json.Unmarshal(bodyAsBytes, &sessionContext)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling the body to session context ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Verify that the prompt and older messages are valid.
		if sessionContext.Question == nil || strings.TrimSpace(*sessionContext.Question) == "" {
			// Cannot continue with empty question
			errMsg := fmt.Sprint("`question` is a required value and should not be empty!")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// If the older messages is not passed, we can set it to an empty array.
		if sessionContext.OlderContext == nil {
			olderContext := make([]ChatGPTMessage, 0)
			sessionContext.OlderContext = &olderContext
		}

		// Add `You're a helpful assistant at the beginning` of the older messages
		// and the new question to the end of the messages.
		updatedOlderMessages := append([]ChatGPTMessage{
			{
				Role:    "system",
				Content: r.GetConfig().GetSystemPrompt(),
			},
		}, *sessionContext.OlderContext...)
		updatedOlderMessages = append(updatedOlderMessages, ChatGPTMessage{Role: "user", Content: *sessionContext.Question})

		// Now that the request is built, we can send it to ChatGPT

		// We will need a sessionId
		sessionId := GenerateSessionId()

		// Since we are not expecting the userId here, we will have to use the
		// user IP address
		userId := iplookup.FromRequest(req)

		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error getting the index names from context"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		// Create the messages interface array
		messagesAsBytes, marshalErr := json.Marshal(updatedOlderMessages)
		if marshalErr != nil {
			errMsg := fmt.Sprint("error while marshalling passed messages to pass to ChatGPT: ", marshalErr.Error())
			log.Errorln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		messagesAsInterfaceArr := make([]map[string]interface{}, 0)
		messageUnmarshalErr := json.Unmarshal(messagesAsBytes, &messagesAsInterfaceArr)
		if messageUnmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling passed messages to pass them to ChatGPT: ", unmarshalErr.Error())
			log.Errorln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// If `model` is passed, use it else set the model value
		// from the config.
		if sessionContext.Model == nil {
			modelToSet := r.GetConfig().GetModel()
			sessionContext.Model = &modelToSet
		}

		// Register the response using the passed sessionId
		r.sessionMap.RegisterResponse(sessionId, userId, indices, make([]string, 0))

		// Fetch the ChatGPT response in a different function and take care of
		// using the sessionId to populate it accordingly.
		log.Debugln(logTag, ": Executing the ChatGPT part of the request in the background")
		go func(sessionContext SessionContext, contextMessages []map[string]interface{}) {
			// Use a deferred function to recover from potential panics
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Recovered from panic in goroutine: %v\n", r)
					debug.PrintStack()
				}
			}()

			err := FetchChatGPTForSessionWithStream(
				sessionId,
				r.sessionMap,
				*sessionContext.Model,
				contextMessages,
				make([]string, 0),
				r.GetConfig().Key(),
				sessionContext.MaxTokens,
				sessionContext.Temperature,
				r.GetConfig().GetAPIType(),
				r.GetConfig().GetAzureURL(),
				r.GetConfig().GetAzureVersion(),
				false,
			)
			// Log the error, if any
			if err != nil {
				log.Printf("Error fetching response from ChatGPT: %v\n", err)
				debug.PrintStack()
			}
		}(sessionContext, messagesAsInterfaceArr)

		responseToReturn := fmt.Sprintf(`{"AIsessionId": "%s"}`, sessionId)
		util.WriteBackRaw(w, []byte(responseToReturn), http.StatusOK)
	}
}

// patchFAQ will support partial updates to the passed FAQ using the ID
func (r *OpenAI) patchFAQ() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		docId := vars["id"]

		// Make sure ID is not empty
		if docId == "" {
			errMsg := fmt.Sprint("`id` is a required param")
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Parse the body and inject the ID
		bodyAsBytes, readErr := ioutil.ReadAll(req.Body)
		if readErr != nil {
			errMsg := fmt.Sprint("error while reading passed body: ", readErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		var FAQBodyPassed FAQBody
		unmarshalErr := json.Unmarshal(bodyAsBytes, &FAQBodyPassed)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling body in FAQBody: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// NOTE: No need to handle the case of local update since local updates will be made through
		// the PUT endpoint instead of this one.

		// Update passed fields in the older body
		updatedBodyWithPassedFields, updateErr := r.UpdatePartialFields(req.Context(), FAQBodyPassed, docId)
		if updateErr != nil {
			errMsg := fmt.Sprint("error while updating fields into older body: ", updateErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// NOTE: No need to update the `faqId` as we will not allow updating
		// that in the update partial fields method.

		// Inject the updated at value
		updatedAt := time.Now().Unix()
		updatedBodyWithPassedFields.UpdatedAt = &updatedAt

		// Store the doc in Zinc
		createErr := r.faqEs.createFAQ(req.Context(), updatedBodyWithPassedFields)
		if createErr != nil {
			errMsg := fmt.Sprint("error while updating the doc in ES: ", createErr.Error())
			log.Warnln(logTag, ": ", errMsg)
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
		// Updating the local state again can cause inconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Send the request to update on other nodes

			// Convert the request body into map[string]interface{}
			reqBody := make(map[string]interface{})
			bodyAsBytes, marshalErr := json.Marshal(updatedBodyWithPassedFields)
			if marshalErr != nil {
				errMsg := fmt.Sprint("error while marshalling FAQ body back into bytes: ", marshalErr.Error())
				log.Errorln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			unmarshalErr := json.Unmarshal(bodyAsBytes, &reqBody)
			if unmarshalErr != nil {
				errMsg := fmt.Sprint("error while unmarshalling the FAQ body into a map: ", unmarshalErr.Error())
				log.Errorln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    fmt.Sprintf("/_ai/faq/%s", *updatedBodyWithPassedFields.ID),
				Body:   reqBody, // forward body
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating FAQ")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		}

		util.WriteBackMessage(w, "FAQ updated successfully", http.StatusOK)
	}
}
