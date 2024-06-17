package rules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/model/acl"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/appbaseio-confidential/reactivesearch/util/iplookup"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
)

func (r *Rules) validateScript() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		ctx := req.Context()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var requestBodyIn ValidateScriptRequestWrapper
		var requestBody ValidateScriptRequest
		var shouldExecuteQuery = false
		var shouldExecuteES = false
		var isDefaultResponse = false

		err2 := json.Unmarshal(reqBody, &requestBodyIn)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
			return
		}

		requestBody, err = extractBodyFromWrapper(requestBodyIn)
		if err != nil {
			// NOTE: Error will be logged by the parent method
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprint("Error while parsing request, ", err.Error()), http.StatusUnprocessableEntity)
			return
		}

		if requestBody.Script == "" {
			telemetry.WriteBackErrorWithTelemetry(req, w, "Script can not be empty", http.StatusBadRequest)
			return
		}

		// We just need the script body to be valid here, everything else can be skipped
		// If it is a cron script, run it right away and return the response
		log.Debug(logTag, pretty.Formatter(requestBody))
		if requestBody.IsCron != nil && *requestBody.IsCron {
			// Pass default timeout for cron scripts
			// i:e 10 mins
			timeout := 10 * 60 * time.Second

			// Create a new context
			cronContext := RuleContext{
				Envs: requestBody.Envs,
			}

			output, err := r.runCronScriptWithTimeout(requestBody.Script, &cronContext, timeout)
			log.Warnln(logTag, "error reported while validating cron script:", err)

			// Marshal the output
			outputMarshalled, err := json.Marshal(output)
			if err != nil {
				log.Warnln(logTag, "error occurred while marshalling cron script output:", err)
				return
			}
			util.WriteBackRaw(w, outputMarshalled, http.StatusOK)
			return
		}

		// Set default request body if not passed by user.
		if requestBody.Request == nil {
			var requestStruct = mockRequest
			requestBody.Request = &ScriptRequest{
				Body: requestStruct,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
			}
		}

		// Set default response body if not passed by user.
		if requestBody.Response == nil {
			requestBody.Response = &ScriptResponse{
				Code: http.StatusOK,
				Headers: map[string]string{
					"Content-Type": "application/json; charset=utf-8",
				},
				Body: mockResponse,
			}
			isDefaultResponse = true
		}

		var parsedRequestBody map[string]interface{}
		requestBodyParseErr := json.Unmarshal([]byte(requestBody.Request.Body), &parsedRequestBody)

		// We want to skip the above error if ACL is set to bulk by the user.
		// However, if the user do not pass any envs at all, we need to show the error
		// since skipping it might cause weird issues.
		//
		// NOTE: If envs are not passed at all, this cannot be a bulk request
		// because bulk requests absolutely require the `envs.acl` set to `bulk`.
		var isBulkRequest = false
		if requestBody.Envs != nil && requestBody.Envs["acl"] == "bulk" {
			isBulkRequest = true
		}

		// If there is an error when the request body is for bulk, just ignore it.
		if requestBodyParseErr != nil && isBulkRequest {
			log.Error(logTag, ": error while unmarshalling request body from string, ", err)
			return
		}

		// Check if the query is to be executed.
		// We will execute the query only if the request is of RS type.
		// RS queries will have a `query` value in the request body.
		//
		// Make sure the request is not bulk because for bulk requests parsedRequestBody
		// would be empty due to unmarshal error above.
		//
		// We don't want execute query to be true if envs were not passed at all since
		// no index is passed.
		if !isBulkRequest && requestBody.Envs != nil && parsedRequestBody["query"] != nil {
			shouldExecuteQuery = true
		}

		// If the acl is one of `bulk`, `create`, `update` or `index`, set the shouldExecuteEs as
		// true since it is an indexing request.
		// NOTE: We don't need to parse the index field here since that will be checked
		// in the child call.
		if requestBody.Envs != nil {
			switch requestBody.Envs["acl"] {
			case "bulk", "create", "update", "index":
				shouldExecuteES = true
			}
		}

		// Add an error message in the response body if envs.index is nil
		// Make sure error message is added only if executeES or executeRS is true.g
		if requestBody.Envs["index"] == nil && (shouldExecuteES || shouldExecuteQuery) {
			var BodyStr = ""
			if shouldExecuteES {
				// NOTE: This body will be overridden if shouldExecuteQuery is true
				// or it is a cron request so no issues passing it like this.
				BodyStr = "{\"message\": \"For request to be indexed, `envs.index` should contain at least one entry and `envs.acl` should be one of: 'bulk', 'create', 'update', 'index'\"}"
			} else if shouldExecuteQuery {
				BodyStr = "{\"message\": \"For request to be searched, `envs.index` should contain at least one entry.\"}"
			}

			requestBody.Response = &ScriptResponse{
				Code:    http.StatusBadRequest,
				Headers: map[string]string{},
				Body:    BodyStr,
			}
		}

		if requestBody.Envs == nil {
			// Extract a RSQuery if the request is like that, else skip
			var environments querytranslate.QueryEnvs

			if parsedRequestBody["query"] != nil {
				// Marshal the request body to bytes and unmarshal into RSQuery again.
				// This is necessary because the request body might be a string.
				marshalledReqBody, err := json.Marshal(requestBody.Request.Body)
				if err != nil {
					log.Errorln("couldn't marshal the request body for RSQuery")
				}
				var reqBody querytranslate.RSQuery
				json.Unmarshal(marshalledReqBody, &reqBody)
				environments = querytranslate.ExtractEnvsFromRequest(reqBody)
			}

			customEvents := make(map[string]string)

			if parsedRequestBody["settings"] != nil && parsedRequestBody["settings"].(map[string]interface{})["customEvents"] != nil {
				for key, value := range parsedRequestBody["settings"].(map[string]interface{})["customEvents"].(map[string]string) {
					customEvents[key] = value
				}
			}

			// Declare default empty strings for passed category
			// and ACL
			var passedCategory = ""
			var passedACL = ""

			// Extract the category from the request context
			reqCategory, err := category.FromContext(ctx)
			if err != nil {
				log.Errorln(logTag, ":", "Couldn't extract category from ctx")
			} else {
				passedCategory = reqCategory.String()
			}

			reqAcl, err := acl.FromContext(ctx)
			if err != nil {
				log.Warnln(logTag, ":", "Couldn't extract ACL from ctx")
			} else {
				passedACL = reqAcl.String()
			}

			ip := iplookup.FromRequest(req)

			var clientIPv4 string
			var clientIPv6 string

			// Try to extract the IP from request
			// If extracting the ipv4 fails, then we don't need to
			// try to extract ipv6.
			ipv4 := GetClientIP4(ip)
			if ipv4 != "" {
				clientIPv4 = ipv4
			} else {
				ipv6 := GetClientIP6(ip)
				if ipv6 != "" {
					clientIPv6 = ipv6
				}
			}

			parsedEnvironments := TriggerEnvironmentsToEvaluate{
				Filter:       ParseFilters(environments.TermFilters),
				Origin:       req.Host,       // https://my-search.domain.com
				Referer:      req.Referer(),  // https://my-search.domain.com/path?q=hello
				Path:         req.URL.Path,   // /_doc/test
				Category:     passedCategory, // docs
				ACL:          passedACL,      // bulk
				IPv4:         clientIPv4,     // 29.120.12.12
				IPv6:         clientIPv6,     // 2001:db8:3333:4444:5555:6666:7777:8888
				CustomEvents: customEvents,
			}
			if environments.Query != nil {
				parsedEnvironments.Query = *environments.Query
			}

			// Update the envs with the extracted ones in the request body.
			requestBody.Envs = TriggerEnvsToMap(parsedEnvironments)
		}

		log.Debug(logTag, ": Passed envs are, ", pretty.Formatter(requestBody.Envs))

		// Pass the default timeout of 5 seconds since we do not
		// expect a trigger type here.
		timeout := 5 * time.Second
		response, err := r.runScript(ScriptContext{
			Request:      *requestBody.Request,
			Response:     *requestBody.Response,
			Environments: requestBody.Envs,
		}, requestBody.Script, shouldExecuteQuery, shouldExecuteES, timeout, isDefaultResponse, false)

		if err != nil {
			log.Errorln(logTag, ":", err)

			// Extract the console logs from the response
			consoleLogs := make([]string, 0)

			if response != nil && response.Response != nil {
				consoleLogs = response.Console
			}

			// Define error code
			code := http.StatusBadRequest

			errBody := map[string]interface{}{
				"error": ValidateScriptError{
					Code:    code,
					Status:  http.StatusText(code),
					Message: err.Error(),
				},
				"console_logs": consoleLogs,
			}

			// Marshal the response
			errBodyRaw, marshalErr := json.Marshal(errBody)
			if marshalErr != nil {
				telemetry.WriteBackErrorWithTelemetry(req, w, "error while marshalling error body", http.StatusInternalServerError)
				return
			}

			// TODO: Write the error to telemetry
			// Is it necessary to write the error to telemetry though?
			util.WriteBackRaw(w, errBodyRaw, code)
			return
		}

		// Convert the response to the wrapper
		responseWrapper, err := getWrapperFromBody(*response, requestBody.Envs["acl"] == "bulk")
		if err != nil {
			errMsg := fmt.Sprint("error while getting wrapper from body, ", err)
			log.Errorln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		marshalledResponse, err := json.Marshal(responseWrapper)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, marshalledResponse, http.StatusOK)
	}
}

func (r *Rules) postRule() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}

		defer req.Body.Close()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			var requestBody ESRuleDoc
			err2 := json.Unmarshal(reqBody, &requestBody)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// Update cache
			AddRuleToCache(requestBody)
			marshalledRule, err := json.Marshal(requestBody)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, marshalledRule, http.StatusCreated)
			return
		}

		var requestBody ESRuleRequestBody
		err2 := json.Unmarshal(reqBody, &requestBody)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
			return
		}

		validateErr := r.validateRule(requestBody)
		if validateErr != nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, validateErr.Error(), http.StatusBadRequest)
			return
		}

		// Size validation starts
		size, err := r.es.getRulesSize(req.Context())
		if err != nil {
			log.Errorln("Error while retrieving the rules size", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		if util.GetTier() != nil {
			if *size >= getSizeLimitByPlan() {
				telemetry.WriteBackErrorWithTelemetry(req, w, "You have reached the maximum rules allowed for the "+util.GetTier().String()+" plan.", http.StatusBadRequest)
				return
			}
		}
		// Size validation ends

		// Add ID
		docID := uuid.New().String()
		requestBody.ID = &docID

		// Add createdAt in doc
		currentTime := time.Now().Unix()
		requestBody.CreatedAt = &currentTime

		// Set default enabled value
		if requestBody.Enabled == nil {
			enabled := true
			requestBody.Enabled = &enabled
		}

		// Set default order
		order := getDefaultRuleOrder(GetRulesFromCache())
		requestBody.Order = &order
		// convert to ES struct where action.data will be stored as string
		esRuleDoc, err3 := requestToESDoc(requestBody)
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			telemetry.WriteBackErrorWithTelemetry(req, w, err3.Error(), http.StatusInternalServerError)
			return
		}
		// Update ES
		err4 := r.es.createRule(req.Context(), docID, esRuleDoc)
		if err4 != nil {
			log.Errorln(logTag, ":", err4)
			telemetry.WriteBackErrorWithTelemetry(req, w, err4.Error(), http.StatusBadRequest)
			return
		}
		// Invoke ACCAPI
		// We're sending the request body as the final parsed struct to be stored in cache
		// The reason for doing is to avoid recreating the properties like `id` and `createdAt` that
		// can lead to inconsistency.
		marshalledRequestBody, err5 := json.Marshal(esRuleDoc)
		if err5 != nil {
			log.Errorln(logTag, ":", err5)
			telemetry.WriteBackErrorWithTelemetry(req, w, err5.Error(), http.StatusBadRequest)
			return
		}

		var bodyJSON map[string]interface{}
		err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
		if err6 != nil {
			log.Errorln(logTag, ":", err6)
			telemetry.WriteBackErrorWithTelemetry(req, w, err6.Error(), http.StatusBadRequest)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPost,
				URL:    "/_rule",
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered creating rule")
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
			AddRuleToCache(esRuleDoc)
		}

		if requestBody.Actions != nil {
			for _, action := range *requestBody.Actions {
				// Replace the script value to rule id
				if action.Type != nil && *action.Type == Script {
					action.Script = requestBody.ID
				}
			}
		}

		// Init the cron rule if it is of that type.
		if *requestBody.Trigger.Type == Cron && *requestBody.Enabled {
			r.initCronRule(esRuleDoc)
		}

		marshalledRule, err := json.Marshal(requestBody)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, marshalledRule, http.StatusCreated)
	}
}

func (r *Rules) putRule() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		ruleID := vars["id"]

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}

		defer req.Body.Close()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			var requestBody ESRuleDoc
			err2 := json.Unmarshal(reqBody, &requestBody)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// Update Cache
			ok := UpdateRuleToCache(ruleID, requestBody)
			if !ok {
				msg := "Error encountered while updating the rule"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
			util.WriteBackMessage(w, "Rule is updated successfully", http.StatusOK)
			return
		}

		var requestBody ESRuleRequestBody
		err2 := json.Unmarshal(reqBody, &requestBody)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
			return
		}

		// Validate rule
		validateErr := r.validateRule(requestBody)
		if validateErr != nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, validateErr.Error(), http.StatusBadRequest)
			return
		}

		// Add updatedAt in doc
		currentTime := time.Now().Unix()
		requestBody.ID = &ruleID
		requestBody.UpdatedAt = &currentTime

		// convert to ES struct where action.data will be stored as string
		esRuleDoc, err3 := requestToESDoc(requestBody)
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			telemetry.WriteBackErrorWithTelemetry(req, w, err3.Error(), http.StatusInternalServerError)
			return
		}
		// Update ES
		err4 := r.es.updateRule(req.Context(), ruleID, esRuleDoc)
		if err4 != nil {
			log.Errorln(logTag, ":", err4)
			telemetry.WriteBackErrorWithTelemetry(req, w, err4.Error(), http.StatusBadRequest)
			return
		}
		// Invoke ACCAPI
		// We're sending the request body as the final parsed struct to be stored in cache
		// The reason for doing is to avoid recreating the properties like `id` and `createdAt` that
		// can lead to inconsistency.
		marshalledRequestBody, err5 := json.Marshal(esRuleDoc)
		if err5 != nil {
			log.Errorln(logTag, ":", err5)
			telemetry.WriteBackErrorWithTelemetry(req, w, err5.Error(), http.StatusBadRequest)
			return
		}
		var bodyJSON map[string]interface{}
		err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
		if err6 != nil {
			log.Errorln(logTag, ":", err6)
			telemetry.WriteBackErrorWithTelemetry(req, w, err6.Error(), http.StatusBadRequest)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_rule/" + ruleID,
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating rule")
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
			ok := UpdateRuleToCache(ruleID, esRuleDoc)
			if !ok {
				msg := "Error encountered while updating the rule"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
		}

		// We need to stop the already running cronjob and start a new one with
		// the updated details.
		if requestBody.Trigger != nil && *requestBody.Trigger.Type == Cron {
			// Make a call to remove the rule
			// If the rule is running then it will be removed else will
			// do nothing.
			r.removeCronRule(ruleID)

			// If it is enabled then init the rule again.
			if *requestBody.Enabled {
				r.initCronRule(esRuleDoc)
			}
		}

		util.WriteBackMessage(w, "Rule is updated successfully", http.StatusOK)
	}
}

func (r *Rules) deleteRule() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		ruleID := vars["id"]
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update Cache
			ok := DeleteRuleToCache(ruleID)
			if !ok {
				msg := "Error encountered while deleting the rule"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
			util.WriteBackMessage(w, "Rule is deleted successfully", http.StatusOK)
			return
		}
		// Delete rule from ES
		err := r.es.deleteRule(req.Context(), ruleID)
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
				URL:    "/_rule/" + ruleID,
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered deleting rule")
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
			ok := DeleteRuleToCache(ruleID)
			if !ok {
				msg := "Error encountered while deleting the rule"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
		}

		r.removeCronRule(ruleID)

		util.WriteBackMessage(w, "Rule is deleted successfully", http.StatusOK)
	}
}

func (r *Rules) getRule() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		ruleID := vars["id"]
		rule := GetRuleFromCache(ruleID)
		if rule != nil {
			parsedRule, err := esDocToRequest(*rule)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			marshalledRule, err := json.Marshal(removeScriptFromRule(parsedRule))
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			util.WriteBackRaw(w, marshalledRule, http.StatusOK)
			return
		}
		telemetry.WriteBackErrorWithTelemetry(req, w, "rule not found for rule id: "+ruleID, http.StatusNotFound)
	}
}

func (r *Rules) getRuleScript() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		ruleID := vars["id"]
		rule := GetRuleFromCache(ruleID)
		if rule != nil {
			var script *string
			if rule.Actions != nil {
				for _, action := range *rule.Actions {
					if action.Type != nil && *action.Type == Script {
						actionScript := action.DecodeScript
						if actionScript != nil {
							script = actionScript
						}
					}
				}
			}
			if script == nil {
				telemetry.WriteBackErrorWithTelemetry(req, w, "script not found for rule id: "+ruleID, http.StatusNotFound)
				return
			}
			response := map[string]interface{}{
				"script": script,
			}
			marshalled, _ := json.Marshal(response)
			util.WriteBackRaw(w, marshalled, http.StatusOK)
			return
		}
		telemetry.WriteBackErrorWithTelemetry(req, w, "rule not found for rule id: "+ruleID, http.StatusNotFound)
	}
}

func (r *Rules) getRules() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		rules := GetRulesFromCache()
		var parsedRules []ESRuleRequestBody
		for _, rule := range rules {
			parsedRule, err := esDocToRequest(rule)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			parsedRules = append(parsedRules, removeScriptFromRule(parsedRule))
		}
		if len(parsedRules) == 0 {
			parsedRules = make([]ESRuleRequestBody, 0)
		}
		marshalledRules, err := json.Marshal(parsedRules)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, marshalledRules, http.StatusOK)
	}
}

func (r *Rules) validateRule(requestBody ESRuleRequestBody) error {

	// Validate actions
	if requestBody.Actions != nil && len(*requestBody.Actions) == 0 {
		return errors.New("actions can't be empty, at least one action must be present for a rule")
	}
	if requestBody.Actions != nil {
		var duplicates = map[string]int{}
		// Validate duplicate actions
		for _, action := range *requestBody.Actions {
			if action.Type == nil {
				return errors.New("type must be present in an action")
			}
			duplicates[action.Type.String()]++

			if duplicates[action.Type.String()] > 1 {
				return errors.New("duplicate actions are not allowed")
			}
		}
	}
	// Validate filter expression for anything other than Always
	if requestBody.Trigger != nil && requestBody.Trigger.Type != nil && *requestBody.Trigger.Type != Always {
		// Expression cannot be empty if type is one of following:
		// - filter
		// - cron
		if (*requestBody.Trigger.Type == Filter || *requestBody.Trigger.Type == Cron) && requestBody.Trigger.Expression == "" {
			return errors.New("expression can't be empty when trigger type is filter or cron")
		}

		// If trigger is not empty, then validate it
		if requestBody.Trigger.Expression != "" {
			// validate expression based on the type of trigger passed
			var err error
			log.Debug(logTag, ":", "Validating the expression based on the trigger")
			switch *requestBody.Trigger.Type {
			case Filter:
				err = validateTriggerExpression(requestBody.Trigger.Expression)
			case Index:
				err = validateIndexTriggerExpression(requestBody.Trigger.Expression)
			case Cron:
				err = validateCronTriggerExpression(requestBody.Trigger.Expression)
			}

			if err != nil {
				return errors.New("invalid trigger expression: " + err.Error())
			}
		}
	}

	// Validate timeframe
	if requestBody.Trigger != nil && requestBody.Trigger.TimeFrame != nil && requestBody.Trigger.TimeFrame.StartTime == nil {
		return errors.New("start_time must be present in timeframe")
	}

	// Validate query string for replace search query rule
	if requestBody.Actions != nil {
		for _, action := range *requestBody.Actions {
			if action.Type != nil && *action.Type == ReplaceSearchQuery {
				// Validate query string
				if action.Data != nil {
					dataAsString, ok := (*action.Data).(string)
					if !ok {
						return errors.New("invalid query value for `replace search query` rule, it must be a string")
					}
					isValid, err := r.es.validateQuery(context.Background(), dataAsString)
					if err != nil {
						return errors.New("error encountered while validating `replace search query` rule: " + err.Error())
					}
					if !isValid {
						return errors.New("invalid query value for `replace search query` rule")
					}
				}
				break
			}
		}
	}

	return nil
}
