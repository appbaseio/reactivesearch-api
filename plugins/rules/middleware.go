package rules

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"

	"strings"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/middleware/validate"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/model/console"
	"github.com/appbaseio-confidential/reactivesearch/model/difference"
	"github.com/appbaseio-confidential/reactivesearch/model/index"
	"github.com/appbaseio-confidential/reactivesearch/model/request"
	"github.com/appbaseio-confidential/reactivesearch/model/requestlogs"
	"github.com/appbaseio-confidential/reactivesearch/model/sourcefilter"
	"github.com/appbaseio-confidential/reactivesearch/model/trackplugin"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
	"github.com/appbaseio-confidential/reactivesearch/plugins/cache"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/buger/jsonparser"
	"github.com/kr/pretty"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
}

// A list of plans for which this feature will be available
var validPlans = []util.Plan{
	util.ArcEnterprise,
	util.HostedArcEnterprise,
	util.ProductionFirst2019, // Note: `function` action won't work with this plan
	util.ProductionSecond2019,
	util.ProductionThird2019,
	util.ProductionFourth2019,
	// 2021 plans
	util.ProductionFirst2021,
	util.ProductionSecond2021,
	util.ProductionThird2021,
	util.HostedArcEnterprise2021,
	util.ProductionFirst2023,
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classifyIndices,
		logs.Recorder(),
		classify.Op(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Operation(),
		validate.Category(),
		validate.Plan(validPlans, util.GetFeatureRules(), "Query Rules feature"),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		rulesCategory := category.Rules

		ctx := category.NewContext(req.Context(), &rulesCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := index.NewContext(req.Context(), []string{defaultRulesEsIndex})
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func (r *Rules) saveRequestToCtx(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body ScriptRequest

		// Extract the body
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(req.Body)
		body.Body = buf.String()

		// Replace original body with the same body
		// since it was emptied when we read it.
		req.Body = ioutil.NopCloser(buf)

		// Extract the headers
		headers := map[string]string{}
		for k := range req.Header {
			headers[k] = req.Header.Get(k)
		}
		body.Headers = headers

		if err != nil {
			log.Errorln(logTag, ": error occurred while reading buffer: ", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf("Can't parse request body: %v", err), http.StatusBadRequest)
			return
		}
		// NOTE: Can't set req.Body to nil because it's used further
		// in the flow.
		// Else it is a good idea to set it to nil to save memory.
		// req.Body = nil
		ctx := NewContext(req.Context(), body)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

// Apply middleware intercepts the reactivesearch requests and applies query rules to the search results.
func Apply() middleware.Middleware {
	return Instance().intercept
}

func (r *Rules) intercept(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		var err error

		c, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error occurred while processing request", http.StatusInternalServerError)
			return
		}
		indexingRequest := isIndexingRequest(req)
		// validate category and indexing type
		if *c != category.ReactiveSearch && !indexingRequest {
			h(w, req)
			return
		}

		requestQuery := new(querytranslate.RSQuery)
		var passedReq *ScriptRequest
		passedReq = new(ScriptRequest)

		if indexingRequest {
			passedReq, err = FromContext(ctx)
			if err != nil {
				log.Error(logTag, ": couldn't extract script request from context.", err)
				return
			}
		} else {
			requestQuery, err = querytranslate.FromContext(ctx)

			// Update the request body with the latest request query body
			parsedRSQuery, err := json.Marshal(requestQuery)
			if err != nil {
				log.Warnln(logTag, "couldn't update RSQuery body in request before applying rules, ", err)
			} else {
				log.Debug(logTag, "Updating request body with latest RSQuery body")
				req.Body = ioutil.NopCloser(strings.NewReader(string(parsedRSQuery)))
			}
		}

		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		// Don't ignore queryRule property
		if requestQuery != nil && requestQuery.Settings != nil && requestQuery.Settings.QueryRule == nil {
			// Don't apply rules if EnableQueryRules set to false in settings
			if requestQuery.Settings != nil &&
				requestQuery.Settings.EnableQueryRules != nil &&
				!*requestQuery.Settings.EnableQueryRules {
				h(w, req)
				return
			}
		}

		rules, err := getFilteredRulesByTrigger(ctx, req, *requestQuery, getTriggerTypeByRequestType(indexingRequest))
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}

		// If the number of rules fetched is 0, don't go ahead in the execution.
		if len(rules) == 0 {
			h(w, req)
			return
		}

		// Extract the disable-logs flag value
		isLogsDisabled, errFetching := difference.FromContext(req.Context())
		if errFetching != nil {
			log.Warnln(logTag, ": error while fetching value of `disable-logs`: ", errFetching.Error())
			defaultLogsDisabled := false
			isLogsDisabled = &defaultLogsDisabled
		}

		var appliedRules []string
		var defaultScriptTook = 0

		// Set it to 0 so it doesn't show up as nil
		var scriptTook *int = &defaultScriptTook
		var requestInfo *request.RequestInfo
		if *c == category.ReactiveSearch {
			rI, err := request.FromRequestIDContext(req.Context())
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request-id from context", http.StatusInternalServerError)
				return
			}
			requestInfo = rI
		}

		// Apply request rules
		for _, rule := range rules {
			if rule.Enabled == nil || *rule.Enabled {
				var sTook *int
				// Records logs
				if requestInfo != nil && !*isLogsDisabled {
					rl := requestlogs.Get(requestInfo.Id)
					if rl != nil {
						rl.LogsDiffing.Add(1)
						go func(request *http.Request, out chan<- requestlogs.LogsResults) {
							// Marshal the body and save it as the one before modification
							copiedBody, err := ioutil.ReadAll(req.Body)
							if err != nil {
								log.Errorln(" error while reading body from request, ", err)
							}
							defer rl.LogsDiffing.Done()
							// Write log output
							out <- requestlogs.LogsResults{
								LogType: "request",
								LogTime: "before",
								Data: requestlogs.RequestData{
									Body:    string(copiedBody),
									Method:  req.Method,
									Headers: req.Header,
									URL:     req.URL.Path,
								},
								Stage: fmt.Sprintf("rule %s", *rule.ID),
							}
						}(req.Clone(req.Context()), rl.Output)
					}
				}

				switch indexingRequest {
				case true:
					sTook, err = applyIndexRequestRule(ctx, req, passedReq, rule)
				default:
					sTook, err = applyRequestRule(ctx, req, requestQuery, rule)
				}
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				if sTook != nil {
					var totalTook *int
					if scriptTook != nil {
						total := *sTook + *scriptTook
						totalTook = &total
					} else {
						totalTook = sTook
					}
					scriptTook = totalTook
				}

				if requestInfo != nil && !*isLogsDisabled {
					rl := requestlogs.Get(requestInfo.Id)
					if rl != nil {
						rl.LogsDiffing.Add(1)
						go func(req *http.Request, timeTaken float64, out chan<- requestlogs.LogsResults) {
							// Marshal the body and save it as the one before modification
							copiedBody, err := ioutil.ReadAll(req.Body)
							if err != nil {
								log.Errorln(" error while reading body from request, ", err)
							}
							defer rl.LogsDiffing.Done()
							// Write log output
							out <- requestlogs.LogsResults{
								LogType: "request",
								LogTime: "after",
								Data: requestlogs.RequestData{
									Body:    string(copiedBody),
									Method:  req.Method,
									Headers: req.Header,
									URL:     req.URL.Path,
								},
								Stage:     fmt.Sprintf("rule %s", *rule.ID),
								TimeTaken: timeTaken,
							}
						}(req.Clone(req.Context()), float64(*scriptTook), rl.Output)
					}
				}

				appliedRules = append(appliedRules, *rule.ID)
			}
		}

		// Update the requestQuery with the latest req.Body
		var updatedRSBody = new(querytranslate.RSQuery)
		// Extract the body
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(req.Body)
		if err != nil {
			log.Warnln(logTag, "error while converting updated body to RSQuery, logs might be corrupted!", err)
		}

		// Unmarshal into RSQuery
		err = json.Unmarshal(buf.Bytes(), updatedRSBody)
		if err != nil {
			log.Warnln(logTag, "Unmarshalling updated body to RSQuery failed, logs might be corrupted!", err)
		} else {
			requestQuery = updatedRSBody
		}

		log.Debug(logTag, "Updated RSQuery body: ", pretty.Formatter(requestQuery))

		// Replace original body with the same body
		// since it was emptied when we read it.
		req.Body = ioutil.NopCloser(buf)

		// Update the RS API request in context
		rsAPIctx := querytranslate.NewContext(req.Context(), *requestQuery)
		req = req.WithContext(rsAPIctx)

		if len(appliedRules) > 0 {
			// Track plugin
			ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "qr")
			req = req.WithContext(ctxTrackPlugin)
		}

		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		// Copy the response to writer
		for k, v := range resp.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.Code)

		// Avoid response modification for cached responses
		if resp.Header().Get(cache.CachedRequestHeader) == "true" {
			w.Write(resp.Body.Bytes())
			return
		}

		// Apply response rules => avoid for RS API validate route
		if !util.IsRSAPIValidateRoute(req) {
			for _, rule := range rules {
				if rule.Enabled == nil || *rule.Enabled {
					var sTook *int

					// Records logs
					if requestInfo != nil && !*isLogsDisabled {
						rl := requestlogs.Get(requestInfo.Id)
						if rl != nil {
							rl.LogsDiffing.Add(1)
							go func(body []byte, headers http.Header, out chan<- requestlogs.LogsResults) {
								defer rl.LogsDiffing.Done()
								// Write log output
								out <- requestlogs.LogsResults{
									LogType: "response",
									LogTime: "before",
									Data: requestlogs.RequestData{
										Body:    string(body),
										Headers: headers,
									},
									Stage: fmt.Sprintf("rule %s", *rule.ID),
								}
							}(resp.Body.Bytes(), resp.HeaderMap, rl.Output)
						}
					}

					switch indexingRequest {
					case true:
						sTook, err = applyIndexResponseRule(ctx, req, resp, passedReq, w, rule)
					default:
						sTook, err = applyResponseRule(ctx, req, resp, requestQuery, w, rule)
					}
					if err != nil {
						log.Errorln(logTag, ":", err)
						telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
						return
					}
					if sTook != nil {
						var totalTook *int
						if scriptTook != nil {
							total := *sTook + *scriptTook
							totalTook = &total
						} else {
							totalTook = sTook
						}
						scriptTook = totalTook
					}
					if !contains(appliedRules, *rule.ID) {
						appliedRules = append(appliedRules, *rule.ID)
					}

					if requestInfo != nil && !*isLogsDisabled {
						rl := requestlogs.Get(requestInfo.Id)
						if rl != nil {
							rl.LogsDiffing.Add(1)
							go func(body []byte, headers http.Header, timeTaken float64, out chan<- requestlogs.LogsResults) {
								defer rl.LogsDiffing.Done()
								// Write log output
								out <- requestlogs.LogsResults{
									LogType: "response",
									LogTime: "after",
									Data: requestlogs.RequestData{
										Body:    string(body),
										Headers: headers,
									},
									Stage:     fmt.Sprintf("rule %s", *rule.ID),
									TimeTaken: timeTaken,
								}
							}(resp.Body.Bytes(), resp.HeaderMap, float64(*scriptTook), rl.Output)
						}
					}

				}
			}
		}
		appliedRulesAsBytes, err := json.Marshal(appliedRules)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		responseToWrite := resp.Body.Bytes()
		// Adds the queryRules applied on request in settings
		// Avoid writing settings for RS API validate route
		if !util.IsRSAPIValidateRoute(req) && len(appliedRules) > 0 {
			responseBody, err := jsonparser.Set(resp.Body.Bytes(), appliedRulesAsBytes, "settings", "queryRules")
			if err != nil {
				log.Warnln(logTag, "unable to set queryRules key in settings", err)
				responseBody2, err2 := jsonparser.Set(resp.Body.Bytes(), []byte(fmt.Sprintf(`{ "queryRules": %s }`, appliedRulesAsBytes)), "settings")
				if err2 != nil {
					log.Warnln(logTag, "unable to set settings key", err2)
				} else {
					responseToWrite = responseBody2
				}
			} else {
				responseToWrite = responseBody
			}
		}
		// apply script took time
		if !util.IsRSAPIValidateRoute(req) && scriptTook != nil {
			responseBody, err := jsonparser.Set(responseToWrite, []byte(fmt.Sprintf("%d", *scriptTook)), "settings", "script_took")
			if err != nil {
				log.Warnln(logTag, "unable to set script_took key in settings", err)
			} else {
				responseToWrite = responseBody
			}
		}
		w.Write(responseToWrite)
	}
}

// Applies the query rules on request
func applyRequestRule(ctx context.Context, originalReq *http.Request, req *querytranslate.RSQuery, rule ESRuleDoc) (*int, error) {
	var err error
	var scriptTook *int
	for _, action := range *rule.Actions {
		if action.Type == nil {
			return nil, nil
		}
		switch *action.Type {
		case ReplaceSearchTerm:
			err = applyReplaceSearch(req, action.Data)
		case ReplaceSearchQuery:
			err = applyReplaceQuery(req, rule, action.Data)
		case RemoveWords:
			err = applyRemoveWords(req, action.Data)
		case ReplaceWords:
			err = applyReplaceWords(req, action.Data)
		case AddFilter:
			err = applyFilters(req, action.Data)
		case SearchSettings:
			err = applySearchSettings(req, action.Data)
		case Script:
			// Convert the passed RSQuery body to a string
			// and pass it along for the rule to be added.
			marshalledBody, err := json.Marshal(req)
			if err != nil {
				log.Error(logTag, ": error while marshalling request body to pass for apply script, ", err)
				return scriptTook, err
			}
			parsedStringBody := string(marshalledBody)

			environments := querytranslate.ExtractEnvsFromRequest(*req)
			scriptTook, err = applyRequestScript(ctx, originalReq, &parsedStringBody, action, environments, Filter)
			if err != nil {
				log.Error(logTag, ": error occurred while applying request rule")
				return scriptTook, err
			}
		}
		if err != nil {
			return scriptTook, err
		}
	}
	return scriptTook, nil
}

// Apply Request Rules for Index.
// As of now, only script is supported.
func applyIndexRequestRule(ctx context.Context, originalReq *http.Request, req *ScriptRequest, rule ESRuleDoc) (*int, error) {
	// As of now only scripts are supported.
	// NOTE: In the future, if more support is added
	// add a switch like the applyRequestRule method.
	var err error
	var scriptTook *int
	for _, action := range *rule.Actions {
		// Create dummy environment
		var environments querytranslate.QueryEnvs
		// NOTE: Assign query to nil so it is skipped while extracting envs
		environments.Query = nil
		scriptTook, err = applyRequestScript(ctx, originalReq, &req.Body, action, environments, Index)
	}
	return scriptTook, err
}

// Applies the query rules on response
func applyResponseRule(
	ctx context.Context,
	req *http.Request,
	res *httptest.ResponseRecorder,
	rsAPI *querytranslate.RSQuery,
	w http.ResponseWriter,
	rule ESRuleDoc,
) (*int, error) {
	var scriptTook *int
	for _, action := range *rule.Actions {
		if action.Type == nil {
			return nil, nil
		}
		var err error
		switch *action.Type {
		case PromoteResult:
			err = applyPromotedResult(req, res, rule, action.Data)
		case HideResult:
			err = applyHideResult(res, rule, action.Data)
		case CustomData:
			err = applyCustomData(res, rule, action.Data)
		case Script:
			// Convert the passed RSQuery body to a string
			// and pass it along for the rule to be added.
			marshalledBody, err := json.Marshal(rsAPI)
			if err != nil {
				log.Error(logTag, ": error while marshalling request body to pass for apply script, ", err)
				return scriptTook, err
			}
			parsedStringBody := string(marshalledBody)

			environments := querytranslate.ExtractEnvsFromRequest(*rsAPI)
			scriptTook, err = applyResponseScript(ctx, req, &parsedStringBody, res, action, environments, Filter)
		}
		if err != nil {
			return nil, err
		}
	}
	return scriptTook, nil
}

// Apply index response rules
func applyIndexResponseRule(
	ctx context.Context,
	req *http.Request,
	res *httptest.ResponseRecorder,
	scriptReq *ScriptRequest,
	w http.ResponseWriter,
	rule ESRuleDoc,
) (*int, error) {
	var scriptTook *int
	for _, action := range *rule.Actions {
		if action.Type == nil {
			return nil, nil
		}
		var err error

		// Right now we only support Script for type
		switch *action.Type {
		case Script:
			// Create a dummy environments and pass it accordingly.
			var environments querytranslate.QueryEnvs
			environments.Query = nil
			scriptTook, err = applyResponseScript(ctx, req, &scriptReq.Body, res, action, environments, Index)
		}

		if err != nil {
			return nil, err
		}
	}
	return scriptTook, nil
}

// Applies the request script
func applyResponseScript(ctx context.Context, originalReq *http.Request, req *string, res *httptest.ResponseRecorder, action Action, environments querytranslate.QueryEnvs, triggerType TriggerType) (*int, error) {
	if action.DecodeScript != nil {
		requestHeaders := map[string]string{}
		for k := range originalReq.Header {
			requestHeaders[k] = originalReq.Header.Get(k)
		}

		interfacedReq, err := FromContext(ctx)
		if err != nil {
			log.Error(logTag, ": Error occurred while extracting rule request: ", err)
			return nil, err
		}

		scriptRequest := ScriptRequest{
			Body:    interfacedReq.Body,
			Headers: requestHeaders,
		}
		systemEnvs, err := getTriggerEnvs(ctx, originalReq, environments, triggerType)
		if err != nil {
			return nil, err
		}
		result := res.Result()
		responseHeaders := map[string]string{}
		for k := range originalReq.Header {
			responseHeaders[k] = originalReq.Header.Get(k)
		}
		var appbaseResponse = res.Body.String()

		scriptResponse := ScriptResponse{
			Code:    result.StatusCode,
			Headers: responseHeaders,
			Body:    appbaseResponse,
		}

		// user environments
		var scriptEnvs = action.Environments
		if scriptEnvs == nil {
			scriptEnvs = make(map[string]interface{})
		}

		systemEnvsBytes, err := json.Marshal(systemEnvs)
		if err != nil {
			return nil, err
		}
		// system environments
		var systemEnvsMap map[string]interface{}
		err2 := json.Unmarshal(systemEnvsBytes, &systemEnvsMap)
		if err2 != nil {
			return nil, err2
		}

		// add system envs to script envs
		for k, v := range systemEnvsMap {
			scriptEnvs[k] = v
		}

		scriptContext := ScriptContext{
			Request:      scriptRequest,
			Response:     scriptResponse,
			Environments: scriptEnvs,
		}
		response, err := singleton.runScript(scriptContext, *action.DecodeScript, false, false, triggerType.Timeout(), true, false)
		if err != nil {
			return nil, err
		}

		if response != nil && response.Response != nil {
			// assign the updated response body
			res.Body = bytes.NewBufferString(response.Response.Body)
			res.Code = response.Response.Code
			// assign response headers
			for k, v := range response.Response.Headers {
				res.Header().Set(k, v)
			}

			// Update console logs after response scripts were run
			currentlogs, err := console.FromContext(originalReq.Context())
			*currentlogs = (*currentlogs)[:0]
			if err != nil {
				log.Warnln(logTag, "error extracting current logs to append new ones, ", err)
			} else {
				consoleStrResponse := console.LimitConsoleString(strings.Join(response.Console, "\n"))
				consoleLogsResponse := strings.Split(consoleStrResponse, "\n")
				*currentlogs = append(*currentlogs, consoleLogsResponse...)
				consoleCtx := console.NewContext(originalReq.Context(), currentlogs)
				originalReq = originalReq.WithContext(consoleCtx)
			}

			return &response.ScriptTook, nil
		}
	}
	return nil, nil
}

func ApplyPromotedResult(requestBody querytranslate.RSQuery, responseBody []byte, data *string) ([]byte, error) {
	if data != nil {
		dataInBytes := []byte(*data)
		var promotedResults []PromotedResult
		err := json.Unmarshal(dataInBytes, &promotedResults)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return responseBody, err
		}
		// filtered promoted results map where key is the query id
		var filteredPromotedResults = make(map[string][]byte)
		keysToIgnore := []string{"_id", "_suggestion_display_value", "_meta"}
		for _, query := range requestBody.Query {
			if !contains(querytranslate.RESERVED_KEYS_IN_RESPONSE, *query.ID) {
				if query.Execute == nil || *query.Execute {
					if (query.IncludeFields != nil && len(*query.IncludeFields) > 0) || (query.ExcludeFields != nil && len(*query.ExcludeFields) > 0) {
						filteredResults := make([]PromotedResult, 0)
						// filter promoted results
						for _, v := range promotedResults {
							filteredDoc := sourcefilter.ApplySourceFiltering(v.Doc, *query.IncludeFields, *query.ExcludeFields)
							if filteredDoc != nil {
								filteredDocAsMap := filteredDoc.(map[string]interface{})
								for _, key := range keysToIgnore {
									if v.Doc[key] != nil {
										filteredDocAsMap[key] = v.Doc[key]
									}
								}
								v.Doc = filteredDocAsMap
								filteredResults = append(filteredResults, v)
							}
						}
						reqBodyBytes := new(bytes.Buffer)
						json.NewEncoder(reqBodyBytes).Encode(filteredResults)
						filteredPromotedResults[*query.ID] = reqBodyBytes.Bytes()
					} else {
						filteredPromotedResults[*query.ID] = dataInBytes
					}
				}
			}

		}

		// write promoted results to response
		copiedResponse := make([]byte, len(responseBody))
		copy(copiedResponse, responseBody)
		err2 := jsonparser.ObjectEach(copiedResponse, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			if !contains(querytranslate.RESERVED_KEYS_IN_RESPONSE, string(key)) {
				var promotedData []byte
				if filteredPromotedResults[string(key)] != nil {
					// Apply filtered results
					promotedData = filteredPromotedResults[string(key)]
				}
				if promotedData != nil {
					for _, query := range requestBody.Query {
						if *query.ID == string(key) && query.Type == querytranslate.Suggestion {
							// append promoted results as top suggestions
							var promotedSuggestions []PromotedResultSuggestion
							err := json.Unmarshal(promotedData, &promotedSuggestions)
							if err != nil {
								return err
							}

							var promoteSuggestions = make([]querytranslate.SuggestionHIT, 0)
							for _, suggestion := range promotedSuggestions {
								source := make(map[string]interface{})
								if suggestion.Doc.Source != nil {
									source = suggestion.Doc.Source
								}
								promoteSuggestions = append(promoteSuggestions, querytranslate.SuggestionHIT{
									Label:  suggestion.Doc.Label,
									Value:  suggestion.Doc.Label,
									URL:    &suggestion.Doc.URL,
									Type:   querytranslate.Promoted,
									Id:     suggestion.Doc.Id,
									Index:  &suggestion.Doc.Index,
									Source: source,
								})
							}

							indexSuggestionsBytes, valueType, _, err := jsonparser.Get(responseBody, *query.ID, "hits", "hits")
							if valueType == jsonparser.NotExist {
								return nil
							}
							if err != nil {
								log.Errorln(logTag, ":", err)
								return nil
							}
							var indexSuggestions []querytranslate.SuggestionHIT
							err2 := json.Unmarshal(indexSuggestionsBytes, &indexSuggestions)
							if err2 != nil {
								log.Errorln(logTag, ":", err2)
								return err2
							}

							// append promoted suggestions at top
							finalSuggestions := append(promoteSuggestions, indexSuggestions...)

							if query.Size != nil && len(finalSuggestions) > *query.Size {
								finalSuggestions = finalSuggestions[:*query.Size]
							}

							finalSuggestionsBytes, err3 := json.Marshal(finalSuggestions)
							if err3 != nil {
								log.Errorln(logTag, ":", err3)
								return err3
							}
							bodyWithRule, err4 := jsonparser.Set(responseBody, finalSuggestionsBytes, *query.ID, "hits", "hits")
							if err4 != nil {
								log.Errorln(logTag, ":", err4)
								return err4
							}
							// Modify total suggestions value
							var err5 error
							response, err5 := jsonparser.Set(bodyWithRule, []byte(strconv.Itoa(len(finalSuggestions))), *query.ID, "hits", "total", "value")
							if err5 != nil {
								log.Errorln(logTag, ":", err5)
								return err5
							}
							responseBody = response
							return nil
						}
					}
					bodyWithRule, err := jsonparser.Set(responseBody, promotedData, string(key), "promoted")
					if err != nil {
						return err
					}
					responseBody = bodyWithRule
				}
				return nil
			}
			return nil
		})
		return responseBody, err2
	}
	return responseBody, nil
}

// Applies the promoted results
func applyPromotedResult(req *http.Request, res *httptest.ResponseRecorder, rule ESRuleDoc, data *string) error {
	if data != nil {
		requestQuery, err := querytranslate.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		if requestQuery != nil {
			copiedResponse := make([]byte, len(res.Body.Bytes()))
			copy(copiedResponse, res.Body.Bytes())
			modifiedResponse, err := ApplyPromotedResult(*requestQuery, copiedResponse, data)
			if err != nil {
				return err
			}
			res.Body = bytes.NewBuffer(modifiedResponse)
			return nil
		}
	}
	return nil
}

// Applies the hide results
// TODO: Not performance efficient when hits size is large
func ApplyHideResult(responseBody []byte, data *string) ([]byte, error) {
	if data != nil {
		dataInBytes := []byte(*data)
		var hiddenResults []string
		err := json.Unmarshal(dataInBytes, &hiddenResults)
		if err != nil {
			log.Println(logTag, ":", err)
			return responseBody, err
		}
		var hiddenDocsMap = make(map[string]bool)
		for _, v := range hiddenResults {
			hiddenDocsMap[v] = true
		}

		copiedResponse := make([]byte, len(responseBody))
		copy(copiedResponse, responseBody)

		err2 := jsonparser.ObjectEach(copiedResponse, func(key []byte, topHitsResponse []byte, dataType jsonparser.ValueType, offset int) error {
			if !contains(querytranslate.RESERVED_KEYS_IN_RESPONSE, string(key)) {
				var hiddenDocs int64 = 0
				var index int = 0
				_, err3 := jsonparser.ArrayEach(topHitsResponse, func(hit []byte, dataType jsonparser.ValueType, offset int, err error) {
					id, err4 := jsonparser.GetString(hit, "_id")
					if err4 != nil {
						log.Warnln(logTag, "error while reading _id from hit", err4)
					} else if hiddenDocsMap[id] {
						// Delete hit
						bodyWithDeletedRule := jsonparser.Delete(responseBody, string(key), "hits", "hits", fmt.Sprintf("[%v]", index))
						responseBody = bodyWithDeletedRule
						hiddenDocs++
						return
					}
					// increase counter
					index += 1
				}, "hits", "hits")
				if err3 != nil {
					log.Errorln(logTag, " : ", err3)
					return fmt.Errorf("error occurred while applying hidden results")
				}

				if hiddenDocs > 0 {
					bodyWithRule, err5 := jsonparser.Set(responseBody, []byte(fmt.Sprintf("%d", hiddenDocs)), string(key), "hidden")
					if err5 != nil {
						log.Errorln(logTag, "error while adding the hidden key", err5)
						return err5
					}
					responseBody = bodyWithRule
				}
				return nil
			}
			return nil
		})
		if err2 != nil {
			log.Errorln(logTag, "error while iterating response", err2)
		}
		return responseBody, err2
	}
	return responseBody, nil
}

// applyHideResult is a wrapper on top of the ApplyHideResult method
// to apply hide result functionality.
func applyHideResult(res *httptest.ResponseRecorder, rule ESRuleDoc, data *string) error {
	if data == nil {
		return nil
	}

	copiedResponse := make([]byte, len(res.Body.Bytes()))
	copy(copiedResponse, res.Body.Bytes())
	modifiedResponse, err := ApplyHideResult(copiedResponse, data)
	if err != nil {
		return err
	}
	res.Body = bytes.NewBuffer(modifiedResponse)
	return nil
}

// Applies the custom data
func ApplyCustomData(responseBody []byte, data *string) ([]byte, error) {
	if data != nil {
		dataInBytes := []byte(*data)
		copiedResponse := make([]byte, len(responseBody))
		copy(copiedResponse, responseBody)
		err2 := jsonparser.ObjectEach(copiedResponse, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
			if !contains(querytranslate.RESERVED_KEYS_IN_RESPONSE, string(key)) {
				bodyWithRule, err := jsonparser.Set(responseBody, dataInBytes, string(key), "customData")
				if err != nil {
					return err
				}
				responseBody = bodyWithRule
				return nil
			}
			return nil
		})
		return responseBody, err2
	}
	return responseBody, nil
}

// Wrap the ApplyCustomData method
func applyCustomData(res *httptest.ResponseRecorder, rule ESRuleDoc, data *string) error {
	if data == nil {
		return nil
	}

	copiedResponse := make([]byte, len(res.Body.Bytes()))
	copy(copiedResponse, res.Body.Bytes())

	modifiedResponse, err := ApplyCustomData(copiedResponse, data)
	if err != nil {
		return err
	}

	res.Body = bytes.NewBuffer(modifiedResponse)
	return nil
}

// Applies the replace search term
func ApplyReplaceSearch(req *querytranslate.RSQuery, data *string) error {
	if data != nil {
		for i, query := range req.Query {
			if query.Type == querytranslate.Search && query.Value != nil {
				valueAsInterface := interface{}(*data)
				req.Query[i].Value = &valueAsInterface
			}
		}
	}
	return nil
}

// Wrapper on top of apply replace search
func applyReplaceSearch(req *querytranslate.RSQuery, data *string) error {
	return ApplyReplaceSearch(req, data)
}

// Applies the request script
func applyRequestScript(ctx context.Context, originalReq *http.Request, req *string, action Action, environment querytranslate.QueryEnvs, triggerType TriggerType) (*int, error) {
	if action.DecodeScript != nil {
		headers := map[string]string{}
		for k := range originalReq.Header {
			headers[k] = originalReq.Header.Get(k)
		}
		scriptRequest := ScriptRequest{
			Body:    *req,
			Headers: headers,
		}
		// user environments
		var scriptEnvs = action.Environments
		if scriptEnvs == nil {
			scriptEnvs = make(map[string]interface{})
		}

		systemEnvs, err := getTriggerEnvs(ctx, originalReq, environment, triggerType)
		if err != nil {
			return nil, err
		}

		systemEnvsBytes, err := json.Marshal(systemEnvs)
		if err != nil {
			return nil, err
		}
		// system environments
		var systemEnvsMap map[string]interface{}
		err2 := json.Unmarshal(systemEnvsBytes, &systemEnvsMap)
		if err2 != nil {
			return nil, err2
		}

		// add system envs to script envs
		for k, v := range systemEnvsMap {
			scriptEnvs[k] = v
		}

		scriptContext := ScriptContext{
			Request:      scriptRequest,
			Environments: scriptEnvs,
		}
		response, err := singleton.runScript(scriptContext, *action.DecodeScript, false, false, triggerType.Timeout(), true, true)
		if err != nil {
			return nil, err
		}
		if response != nil && response.Request != nil {
			// assign the updated body
			(*req) = response.Request.Body

			// Assign the updated request body to original Request

			originalReq.Body = ioutil.NopCloser(strings.NewReader(response.Request.Body))

			// Update the console logs after the request rules are applied
			// Extract the logs from context, empty array and add new values
			currentlogs, err := console.FromContext(originalReq.Context())
			*currentlogs = (*currentlogs)[:0]
			if err != nil {
				log.Warnln(logTag, "error extracting current logs to append new ones, ", err)
			} else {
				consoleStr := console.LimitConsoleString(strings.Join(response.Console, "\n"))
				consoleLogs := strings.Split(consoleStr, "\n")
				*currentlogs = append(*currentlogs, consoleLogs...)
				consoleCtx := console.NewContext(originalReq.Context(), currentlogs)
				originalReq = originalReq.WithContext(consoleCtx)
			}

			// assign request headers
			for k, v := range response.Request.Headers {
				originalReq.Header.Set(k, v)
			}
			return &response.ScriptTook, nil
		}
	}
	return nil, nil
}

// Applies the replace search query rule
func applyReplaceQuery(req *querytranslate.RSQuery, rule ESRuleDoc, data *string) error {
	if data != nil {
		// Extract regexp from the trigger expression
		if rule.Trigger == nil || rule.Trigger.Expression == "" {
			return nil
		}
		regexFrmTriggerExp := getRegexpFromTriggerExp(rule.Trigger.Expression)
		if regexFrmTriggerExp == "" {
			return nil
		}
		// Replace unnecessary escape chars
		r, err := regexp.Compile(strings.Replace(regexFrmTriggerExp, `\\`, `\`, -1))
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil
		}
		log.Println(logTag, "Compiled Regex: ", r)
		for i, query := range req.Query {
			// Only apply to search type of queries
			if query.Type == querytranslate.Search && query.Value != nil {
				// Parse the query value by using capturing gropus from trigger expression
				queryAsString, isString := (*query.Value).(string)
				if !isString {
					continue
				}
				// Capture group values
				subMatches := r.FindStringSubmatch(queryAsString)
				if subMatches == nil {
					continue
				}
				log.Println(logTag, "Captured Regex Groups: ", subMatches)
				// Parse the data and assign it to the query's value
				// For an e.g "id:$1 + od:$2 + height:$3"
				var queryStringValue string
				// TODO: Replace with high perf json marshal library
				err := json.Unmarshal([]byte(*data), &queryStringValue)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil
				}
				for index, value := range subMatches {
					// ignore the 0th index because it'll be the full string
					if index == 0 {
						continue
					}
					varNum := strconv.Itoa(index)
					// Find and replace variables in data
					queryStringValue = strings.ReplaceAll(queryStringValue, "$"+varNum, value)
					log.Println(logTag, "Compiled Search Query: ", queryStringValue)
				}
				valueAsInterface := interface{}(queryStringValue)
				req.Query[i].Value = &valueAsInterface
				// Set query string to true
				queryString := true
				req.Query[i].QueryString = &queryString
			}
		}
	}
	return nil
}

func ApplyRemoveWords(req *querytranslate.RSQuery, data *string) error {
	if data != nil {
		dataInBytes := []byte(*data)
		var words []string
		// TODO: Replace with high perf json marshal library
		err := json.Unmarshal(dataInBytes, &words)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		for i, query := range req.Query {
			if query.Type == querytranslate.Search && query.Value != nil {
				stringValue := (*query.Value).(string)
				for _, word := range words {
					stringValue = strings.ReplaceAll(stringValue, word, "")
				}
				valueAsInterface := interface{}(stringValue)
				req.Query[i].Value = &valueAsInterface
			}
		}
	}
	return nil
}

// applyRemoveWords is a wrapper on top of ApplyRemoveWords
func applyRemoveWords(req *querytranslate.RSQuery, data *string) error {
	return ApplyRemoveWords(req, data)
}

func ApplyReplaceWords(req *querytranslate.RSQuery, data *string) error {
	if data != nil {
		dataInBytes := []byte(*data)
		var replaceConfig map[string]string
		// TODO: Replace with high perf json marshal library
		err := json.Unmarshal(dataInBytes, &replaceConfig)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		for i, query := range req.Query {
			if query.Type == querytranslate.Search && query.Value != nil {
				stringValue, ok := (*query.Value).(string)
				if !ok {
					log.Errorln(logTag, "Error encountered while parsing search value to string")
					continue
				}
				for k, v := range replaceConfig {
					stringValue = strings.ReplaceAll(stringValue, k, v)
				}
				valueAsInterface := interface{}(stringValue)
				req.Query[i].Value = &valueAsInterface
			}
		}
	}
	return nil
}

// applyReplaceWords is a wrapper on top of ApplyReplaceWords
func applyReplaceWords(req *querytranslate.RSQuery, data *string) error {
	return ApplyReplaceWords(req, data)
}

// modifies the `and` prop in react property of query (add facet)
func applyReactAndProp(componentID string, queryIndex int, react interface{}) {
	reactAsMap, isReactAsMap := (react).(map[string]interface{})
	if isReactAsMap {
		if reactAsMap["and"] != nil {
			andValue := reactAsMap["and"]
			valueAsArray, ok := (andValue).([]interface{})
			if ok {
				reactAsMap["and"] = append(valueAsArray, componentID)
			} else {
				// handle value as string
				valueAsString, ok := (andValue).(string)
				if ok {
					reactAsMap["and"] = []interface{}{valueAsString, componentID}
				} else {
					reactAndAsMap, isReactAndAsMap := (andValue).(map[string]interface{})
					if isReactAndAsMap && reactAndAsMap["and"] != nil {
						// iterate till the leaf node
						applyReactAndProp(componentID, queryIndex, reactAndAsMap)
					}
				}
			}
		} else {
			reactAsMap["and"] = componentID
		}
	} else {
		react = map[string]interface{}{
			"and": componentID,
		}
	}
}

type SearchSettingsConfig struct {
	DataField    *[]string  `json:"dataField"`
	FieldWeights *[]float64 `json:"fieldWeights"`
}

// Applies the search settings
func ApplySearchSettings(req *querytranslate.RSQuery, data *string) error {
	if data != nil {
		dataInBytes := []byte(*data)
		var searchSettings SearchSettingsConfig
		// TODO: Replace with high perf json marshal library
		err := json.Unmarshal(dataInBytes, &searchSettings)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		for queryIndex, query := range req.Query {
			if query.Type == querytranslate.Search && query.Value != nil {
				// Apply the dataField on search query
				if searchSettings.DataField != nil {
					req.Query[queryIndex].DataField = *searchSettings.DataField
				}
				// Apply the fieldWeights on search query
				if searchSettings.FieldWeights != nil {
					req.Query[queryIndex].FieldWeights = *searchSettings.FieldWeights
				}
			}
		}
	}
	return nil
}

func applySearchSettings(req *querytranslate.RSQuery, data *string) error {
	return ApplySearchSettings(req, data)
}

// Applies the filters
func ApplyFilters(req *querytranslate.RSQuery, data *string) error {
	if data != nil {
		dataInBytes := []byte(*data)
		var filters map[string]interface{}
		// TODO: Replace with high perf json marshal library
		err := json.Unmarshal(dataInBytes, &filters)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		// tracks that if a filter query is added to the request body or not
		var filterQueryCache = make(map[string]interface{})
		for queryIndex, query := range req.Query {
			if query.Type == querytranslate.Search {
				for filterKey, filterValue := range filters {
					id := "query_rule_filter_" + filterKey

					react := query.React
					if query.React != nil {
						applyReactAndProp(id, queryIndex, *query.React)
					} else {
						react = &map[string]interface{}{
							"and": id,
						}
					}
					// Apply the react prop on query
					req.Query[queryIndex].React = react
					if filterQueryCache[id] == nil {
						var value interface{} = filterValue
						// Don't execute the filter query
						execute := false
						// Add the term query
						termQuery := querytranslate.Query{
							ID:        &id,
							Type:      querytranslate.Term,
							DataField: []string{filterKey},
							Value:     &value,
							Execute:   &execute,
						}
						// append query
						req.Query = append(req.Query, termQuery)
						// Update cache
						filterQueryCache[id] = true
					}
				}
				// only apply on the first search query
				break
			}
		}
	}
	return nil
}

func applyFilters(req *querytranslate.RSQuery, data *string) error {
	return ApplyFilters(req, data)
}

// getFilteredRulesByTrigger returns the filtered rules by trigger condition
func getFilteredRulesByTrigger(ctx context.Context, r *http.Request, req querytranslate.RSQuery, triggerType TriggerType) ([]ESRuleDoc, error) {
	var filteredRules []ESRuleDoc
	var rules = []ESRuleDoc{}

	isRulesDisabled := false
	if req.Settings != nil &&
		req.Settings.EnableQueryRules != nil &&
		!*req.Settings.EnableQueryRules {
		isRulesDisabled = true
	}
	if !isRulesDisabled {
		cachedRules := GetRulesFromCache()
		copiedRules := make([]ESRuleDoc, len(cachedRules))
		// copy rules from cache
		copy(copiedRules, cachedRules)
		rules = copiedRules
	}

	// Filter out and keep only those rules that are
	// of the passed triggerType
	rules = filterRulesByTriggerType(rules, triggerType)

	// apply rule from settings
	if req.Settings != nil && req.Settings.QueryRule != nil {
		marshalledRule, err := json.Marshal(*req.Settings.QueryRule)
		if err != nil {
			return filteredRules, err
		}
		var rule ESRuleRequestBody
		err2 := json.Unmarshal(marshalledRule, &rule)
		if err2 != nil {
			return filteredRules, err2
		}
		esRuleDoc, err3 := requestToESDoc(rule)
		if err3 != nil {
			return filteredRules, err3
		}
		// use default id if not present
		if esRuleDoc.ID == nil {
			var ruleID = defaultSettingsRuleID
			esRuleDoc.ID = &ruleID
		}
		// add order at end
		if esRuleDoc.Order == nil {
			ruleOrder := 0
			esRuleDoc.Order = &ruleOrder
		}
		rules = append(rules, esRuleDoc)
	}
	// sort rules by order
	sort.Slice(rules, func(i, j int) bool {
		return *rules[i].Order < *rules[j].Order
	})
	// iterate rules in reverse order
	for i := len(rules) - 1; i >= 0; i-- {
		rule := rules[i]
		// Don't evaluate expression for disabled functions
		if rule.Enabled != nil && !*rule.Enabled {
			continue
		}
		// Validate timeframe
		ok := validateTimeframe(rule)
		if !ok {
			continue
		}
		// Validate filter expression
		ok, err := validateFilter(ctx, r, rule)
		if err != nil {
			return filteredRules, err
		}
		if ok {
			filteredRules = append(filteredRules, rule)
		}
	}
	return filteredRules, nil
}

// Filter the passed list of rules based on the
// passed trigger and accordingly return the filtered
// out rules.
// NOTE: Always will always be included in
// the filtered rules.
func filterRulesByTriggerType(rules []ESRuleDoc, triggerType TriggerType) []ESRuleDoc {
	var filteredRules []ESRuleDoc

	for _, rule := range rules {
		if *rule.Trigger.Type == triggerType || *rule.Trigger.Type == Always {
			filteredRules = append(filteredRules, rule)
		}
	}

	return filteredRules
}

// Get the trigger type based on the request type.
// There are two possible trigger types: Filter and Index
// If the request is an indexing request, Index will be
// returned, else Filter.
func getTriggerTypeByRequestType(isIndexingRequest bool) TriggerType {
	if isIndexingRequest {
		return Index
	} else {
		return Filter
	}
}
