package suggestions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/difference"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/request"
	"github.com/appbaseio/reactivesearch-api/model/requestlogs"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/plugins/uibuilder"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
)

// A list of plans for which this feature will be available
var validPlans = []util.Plan{
	util.ArcEnterprise,
	util.HostedArcEnterprise,
	util.ProductionFirst2019,
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

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classify.Op(),
		classify.Indices(),
		logs.Recorder(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
		validate.Plan(validPlans, util.GetFeatureSuggestions(), "Suggestions feature"),
		telemetry.Recorder(),
	}
}

type SuggestionOutput struct {
	QueryID     string
	Suggestions []querytranslate.SuggestionHIT
	Error       *Error
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.Suggestions

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func (rx *suggestions) intercept(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		c, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error occurred while processing request", http.StatusInternalServerError)
			return
		}
		// validate category
		if *c != category.ReactiveSearch {
			h(w, req)
			return
		}

		ctxIndices, err := index.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ": cannot fetch indices from request context, ", err)
			return
		}

		requestQuery, err := querytranslate.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}
		enableSearchRelevancy := true
		// don't apply search relevancy if disabled
		if requestQuery.Settings != nil &&
			requestQuery.Settings.EnableSearchRelevancy != nil {
			enableSearchRelevancy = *requestQuery.Settings.EnableSearchRelevancy
		}

		requestInfo, err := request.FromRequestIDContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request-id from context", http.StatusInternalServerError)
			return
		}

		stage := "suggestion"

		// Extract the disable-logs flag value
		isLogsDisabled, errFetching := difference.FromContext(req.Context())
		if errFetching != nil {
			log.Warnln(logTag, ": error while fetching value of `disable-logs`: ", errFetching.Error())
			defaultLogsDisabled := false
			isLogsDisabled = &defaultLogsDisabled
		}

		// We will capture this stage logs only if the type is suggestion
		// for at least one query.
		isSuggestionApplied := false
		if enableSearchRelevancy {
			// Apply suggestions preferences

			// Start timer
			start := time.Now()

			// Copy the request body before modification
			// NOTE: The modifications are made to the RSQuery body
			// so we don't need to calculate diff for headers etc.
			var shouldLogDiff = true

			for _, query := range requestQuery.Query {
				if query.Type == querytranslate.Suggestion {
					isSuggestionApplied = true
				}
			}
			if requestInfo != nil && !*isLogsDisabled {
				// Records logs
				rl := requestlogs.Get(requestInfo.Id)
				if isSuggestionApplied {
					// Records logs
					if rl != nil {
						rl.LogsDiffing.Add(1)
						go func(request *querytranslate.RSQuery, out chan<- requestlogs.LogsResults) {
							// Marshal the body and save it as the one before modification
							marshalledReqBody, err := json.Marshal(request)
							if err != nil {
								log.Warnln(logTag, "error while marshalling request query, ", err)
								shouldLogDiff = false
							}
							defer rl.LogsDiffing.Done()
							// Write log output
							out <- requestlogs.LogsResults{
								LogType: "request",
								LogTime: "before",
								Data: requestlogs.RequestData{
									Body:    string(marshalledReqBody),
									Method:  req.Method,
									Headers: req.Header,
									URL:     req.URL.Path,
								},
								Stage: stage,
							}
						}(requestQuery, rl.Output)
					}
				}
			}

			ApplySuggestionsPreferences(*requestQuery, &Preferences{
				Index:   GetIndexPreferences(),
				Popular: GetPopularPreferences(),
				Recent:  GetRecentPreferences(),
			})

			if shouldLogDiff && isSuggestionApplied && !*isLogsDisabled {
				if requestInfo != nil {
					// Records logs
					rl := requestlogs.Get(requestInfo.Id)
					if rl != nil {
						rl.LogsDiffing.Add(1)
						timeTaken := float64(int(time.Since(start).Milliseconds()))
						go func(request *querytranslate.RSQuery, timeTaken float64, out chan<- requestlogs.LogsResults) {
							// Marshal the body and save it as the one before modification
							marshalledReqBody, err := json.Marshal(request)
							if err != nil {
								log.Warnln(logTag, "error while marshalling request query, ", err)
								shouldLogDiff = false
							}
							defer rl.LogsDiffing.Done()
							// Write log output
							out <- requestlogs.LogsResults{
								LogType: "request",
								LogTime: "after",
								Data: requestlogs.RequestData{
									Body:    string(marshalledReqBody),
									Method:  req.Method,
									Headers: req.Header,
									URL:     req.URL.Path,
								},
								Stage:     stage,
								TimeTaken: timeTaken,
							}
						}(requestQuery, float64(timeTaken), rl.Output)
					}
				}

			}
		}

		rsAPIctx := querytranslate.NewContext(req.Context(), *requestQuery)
		req = req.WithContext(rsAPIctx)

		// Track plugin
		ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "su")
		req = req.WithContext(ctxTrackPlugin)

		// already verified that category is of type reactivesearch
		// if it's a validate route additionally, we don't need to manipulate the response
		if strings.Contains(req.URL.EscapedPath(), "/validate") {
			// Generate the document suggestions query if it is enabled and accordingly
			// wait for the response to inject the query.
			log.Debug(logTag, ": validate call is detected")

			// Wait for the response to arrive
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)

			// Copy the response to writer
			for k, v := range resp.Header() {
				w.Header()[k] = v
			}

			// Check if error was already processed
			if resp.Header().Get("X-Error-Processed") == "true" {
				// Remove the header before writing
				resp.Header().Del("X-Error-Processed")
				w.WriteHeader(resp.Code)
				w.Write(resp.Body.Bytes())
				return
			}
			response := resp.Body.Bytes()

			// Modify the response here

			// Since the response is an array of objects, we will unmarshal
			// Try to unmarshal as an array first
			responseAsArr := make([]interface{}, 0)
			unmarshalErr := json.Unmarshal(response, &responseAsArr)

			// If it fails, try to unmarshal as an object and convert to an array
			if unmarshalErr != nil {
				log.Debugln(logTag, ": Failed to unmarshal as array, trying as object: ", unmarshalErr.Error())

				// Try to unmarshal as object instead
				responseAsObj := make(map[string]interface{})
				objUnmarshalErr := json.Unmarshal(response, &responseAsObj)
				if objUnmarshalErr != nil {
					errMsg := fmt.Sprintf("error unmarshaling response as array or object: %v", objUnmarshalErr.Error())
					log.Errorln(logTag, ": ", errMsg)
					telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
					return
				}

				// Convert the object to a single-element array
				responseAsArr = []interface{}{responseAsObj}
			}
			originalResponseLen := len(responseAsArr)

			// Determine the userId. Try to parse it from the settings else
			// fallback to the user's IP address.
			validateUserId := iplookup.FromRequest(req)
			if requestQuery.Settings != nil && requestQuery.Settings.UserID != nil && strings.TrimSpace(*requestQuery.Settings.UserID) != "" {
				validateUserId = *requestQuery.Settings.UserID
			}

			for _, query := range requestQuery.Query {
				if (query.Execute == nil || *query.Execute) &&
					query.Type == querytranslate.Suggestion &&
					query.EnableDocumentSuggestions != nil &&
					*query.EnableDocumentSuggestions {
					// Generate the document suggestions query

					var value string
					if query.Value != nil {
						valueAsString, ok := (*query.Value).(string)
						if ok {
							value = valueAsString
						}
					}

					documentsConfig := query.DocumentSuggestionsOptions

					if documentsConfig == nil {
						documentsConfig = &querytranslate.RecentDocumentSuggestionsOptions{}
					}

					_, finalQuery := GenerateDocumentSuggestionsQuery(query, value, *documentsConfig, validateUserId, ctxIndices)
					if finalQuery == nil {
						// Skip generating the document suggestions query
						log.Infoln(logTag, ": query not generated, probably because length of value is more than maxChars.")
						continue
					}

					// Capture the query so it can be returned
					generatedQuery, fetchErr := finalQuery.Source()
					if fetchErr != nil {
						log.Warnln(logTag, ": error while getting the generated query from elasticsearch library: ", fetchErr.Error())
						continue
					}

					// Extract the headers passed with the current request without the
					// authorization header
					headersPassed := make(map[string]interface{})
					for key, value := range req.Header {
						if strings.ToLower(key) == "authorization" {
							continue
						}

						headersPassed[key] = strings.Join(value, ", ")
					}

					responseAsArr = append(responseAsArr, map[string]interface{}{
						"id": *query.ID,
						"endpoint": map[string]interface{}{
							"body":    generatedQuery,
							"method":  "POST",
							"url":     util.CleanPasswordFromURL(fmt.Sprint(util.GetESURL(), "/.documents/_search")),
							"headers": headersPassed,
						},
					})
				}
			}

			// If the response was modified, marshal it to bytes again
			if originalResponseLen != len(responseAsArr) {
				var responseMarshalErr error
				response, responseMarshalErr = json.Marshal(responseAsArr)
				if responseMarshalErr != nil {
					errMsg := fmt.Sprint("error while marshaling updated response back to bytes: ", responseMarshalErr.Error())
					log.Errorln(logTag, ": ", errMsg)
					telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
					return
				}
			}

			w.WriteHeader(resp.Code)
			w.Write(response)

			return
		}

		// Start timer for response changes
		start := time.Now()

		var popularSuggestionsWg sync.WaitGroup
		popularSuggestionsOut := make(chan SuggestionOutput)

		var recentSuggestionsWg sync.WaitGroup
		recentSuggestionsOut := make(chan SuggestionOutput)

		var featuredSuggestionsWg sync.WaitGroup
		featuredSuggestionsOut := make(chan SuggestionOutput)

		var faqSuggestionsWg sync.WaitGroup
		faqSuggestionsOut := make(chan SuggestionOutput)

		var recentDocumentSuggestionsWg sync.WaitGroup
		recentDocumentsSuggestionsOut := make(chan SuggestionOutput)

		// fetch popular and recent suggestions
		for _, query := range requestQuery.Query {
			if query.Type == querytranslate.Suggestion {
				// calculate featured suggestions
				var value string
				if query.Value != nil {
					if valueAsString, ok := (*query.Value).(string); ok {
						value = valueAsString
					}
				}
				featuredSuggestionsWg.Add(1)
				go func(out chan<- SuggestionOutput, query querytranslate.Query, value string) {
					defer featuredSuggestionsWg.Done()
					featuredSuggestions, err := uibuilder.GetFeaturedSuggestions(&query, value)
					if err != nil {
						log.Errorln(logTag, ":", err)
						out <- SuggestionOutput{
							QueryID: *query.ID,
							Error:   &Error{Error: err},
						}
					} else {
						out <- SuggestionOutput{
							QueryID:     *query.ID,
							Suggestions: featuredSuggestions,
							Error:       nil,
						}
					}
				}(featuredSuggestionsOut, query, value)

				// fetch popular suggestions
				if query.EnablePopularSuggestions != nil &&
					*query.EnablePopularSuggestions {
					popularSuggestionsWg.Add(1)
					go func(out chan<- SuggestionOutput, psConfig *querytranslate.PopularSuggestionsOptions, query querytranslate.Query) {
						defer popularSuggestionsWg.Done()
						var value string
						if query.Value != nil {
							valueAsString, ok := (*query.Value).(string)
							if ok {
								value = valueAsString
							}
						}
						config := querytranslate.PopularSuggestionsOptions{}
						if psConfig != nil {
							config = *psConfig
						}
						popularSuggestions, err := GetPopularSuggestions(config, value, ctxIndices)
						if err != nil {
							log.Errorln(logTag, ":", err)
							out <- SuggestionOutput{
								QueryID: *query.ID,
								Error:   err,
							}
						} else {
							out <- SuggestionOutput{
								QueryID:     *query.ID,
								Suggestions: popularSuggestions,
								Error:       nil,
							}
						}
						log.Debug(logTag, ": id: ", *query.ID)
					}(popularSuggestionsOut, query.PopularSuggestionsConfig, query)
				}

				// fetch FAQ suggestions
				if query.EnableFAQSuggestions != nil &&
					*query.EnableFAQSuggestions {
					faqSuggestionsWg.Add(1)
					go func(out chan<- SuggestionOutput, FAQConfig *querytranslate.FAQSuggestionsOptions, query querytranslate.Query) {
						defer faqSuggestionsWg.Done()
						var value string
						if query.Value != nil {
							valueAsString, ok := (*query.Value).(string)
							if ok {
								value = valueAsString
							}
						}

						if FAQConfig == nil {
							FAQConfig = &querytranslate.FAQSuggestionsOptions{}
						}

						faqSearchBoxId := query.FAQSearchBoxId
						if *query.FAQSearchBoxId == "" {
							faqSearchBoxId = query.SearchBoxId
						}
						// Make the call to get FAQ suggestions based on the query
						faqSuggestions, err := GetFAQSuggestions(*FAQConfig, value, faqSearchBoxId)
						if err != nil {
							log.Errorln(logTag, ":", err)
							out <- SuggestionOutput{
								QueryID: *query.ID,
								Error:   err,
							}
						} else {
							out <- SuggestionOutput{
								QueryID:     *query.ID,
								Suggestions: faqSuggestions,
								Error:       nil,
							}
						}
					}(faqSuggestionsOut, query.FAQSuggestionsConfig, query)
				}

				// fetch recentDocumentsSuggestions
				if query.EnableDocumentSuggestions != nil && *query.EnableDocumentSuggestions {
					recentDocumentSuggestionsWg.Add(1)
					go func(out chan<- SuggestionOutput, documentsConfig *querytranslate.RecentDocumentSuggestionsOptions, query querytranslate.Query) {
						defer recentDocumentSuggestionsWg.Done()
						var value string
						if query.Value != nil {
							valueAsString, ok := (*query.Value).(string)
							if ok {
								value = valueAsString
							}
						}

						// Determine the userId. Try to parse it from the settings else
						// fallback to the user's IP address.
						userId := iplookup.FromRequest(req)
						if requestQuery.Settings != nil && requestQuery.Settings.UserID != nil && strings.TrimSpace(*requestQuery.Settings.UserID) != "" {
							userId = *requestQuery.Settings.UserID
						}

						if documentsConfig == nil {
							documentsConfig = &querytranslate.RecentDocumentSuggestionsOptions{}
						}

						// Make the call to get FAQ suggestions based on the query
						documentSuggestions, err := GetDocumentSuggestions(query, value, *documentsConfig, userId, ctxIndices)
						if err != nil {
							log.Errorln(logTag, ":", err)
							out <- SuggestionOutput{
								QueryID: *query.ID,
								Error:   err,
							}
						} else {
							out <- SuggestionOutput{
								QueryID:     *query.ID,
								Suggestions: documentSuggestions,
								Error:       nil,
							}
						}
					}(recentDocumentsSuggestionsOut, query.DocumentSuggestionsOptions, query)
				}

				// fetch recent suggestions
				if query.EnableRecentSuggestions != nil &&
					*query.EnableRecentSuggestions {
					recentSuggestionsWg.Add(1)
					go func(query querytranslate.Query, out chan<- SuggestionOutput, rcConfig *querytranslate.RecentSuggestionsOptions) {
						defer recentSuggestionsWg.Done()
						config := querytranslate.RecentSuggestionsOptions{}
						if rcConfig != nil {
							config = *rcConfig
						}
						recentSuggestions, err := GetRecentSuggestions(query, config, ctxIndices)
						if err != nil {
							log.Errorln(logTag, ":", err)
							out <- SuggestionOutput{
								QueryID: *query.ID,
								Error:   err,
							}
						} else {
							out <- SuggestionOutput{
								QueryID:     *query.ID,
								Suggestions: recentSuggestions,
								Error:       nil,
							}
						}
					}(query, recentSuggestionsOut, query.RecentSuggestionsConfig)
				}
			}
		}
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		// Copy the response to writer
		for k, v := range resp.Header() {
			w.Header()[k] = v
		}

		response := resp.Body.Bytes()

		// Records logs
		if isSuggestionApplied {
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
							Stage: stage,
						}
					}(response, resp.HeaderMap, rl.Output)
				}
			}

		}

		w.WriteHeader(resp.Code)
		// wait for popular suggestions
		go func() {
			popularSuggestionsWg.Wait()
			close(popularSuggestionsOut)
		}()
		// popular suggestions to query ID map
		var popularSuggestionsMap = make(map[string]SuggestionOutput)
		for result := range popularSuggestionsOut {
			popularSuggestionsMap[result.QueryID] = result
		}
		// wait for recent suggestions
		go func() {
			recentSuggestionsWg.Wait()
			close(recentSuggestionsOut)
		}()
		// popular suggestions to query ID map
		var recentSuggestionsMap = make(map[string]SuggestionOutput)
		for result := range recentSuggestionsOut {
			recentSuggestionsMap[result.QueryID] = result
		}
		// wait for featured suggestions
		go func() {
			featuredSuggestionsWg.Wait()
			close(featuredSuggestionsOut)
		}()
		// featured suggestions to query ID map
		var featuredSuggestionsMap = make(map[string]SuggestionOutput)
		for result := range featuredSuggestionsOut {
			featuredSuggestionsMap[result.QueryID] = result
		}

		// wait for faq suggestions
		go func() {
			faqSuggestionsWg.Wait()
			close(faqSuggestionsOut)
		}()
		// faq suggestions to query ID map
		var faqSuggestionsMap = make(map[string]SuggestionOutput)
		for result := range faqSuggestionsOut {
			faqSuggestionsMap[result.QueryID] = result
		}

		// wait for document suggestions
		go func() {
			recentDocumentSuggestionsWg.Wait()
			close(recentDocumentsSuggestionsOut)
		}()
		// faq suggestions to query ID map
		var documentSuggestionsMap = make(map[string]SuggestionOutput)
		for result := range recentDocumentsSuggestionsOut {
			documentSuggestionsMap[result.QueryID] = result
		}

		// apply popular suggestions
		responseWithSuggestions, err2 := ApplySuggestions(*requestQuery, response, recentSuggestionsMap, popularSuggestionsMap, featuredSuggestionsMap, faqSuggestionsMap, documentSuggestionsMap)
		if err2 != nil {
			code := http.StatusInternalServerError
			if err2.Code != 0 {
				code = err2.Code
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error.Error(), code)
			return
		}
		response = responseWithSuggestions
		if isSuggestionApplied {
			if requestInfo != nil && !*isLogsDisabled {
				rl := requestlogs.Get(requestInfo.Id)
				timeTaken := float64(int(time.Since(start).Milliseconds()))
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
							Stage:     stage,
							TimeTaken: timeTaken,
						}
					}(response, resp.HeaderMap, float64(timeTaken), rl.Output)
				}
			}

		}

		w.Write(response)
	}
}

type Preferences struct {
	Index   IndexPreferences   `json:"index"`
	Popular PopularPreferences `json:"popular"`
	Recent  RecentPreferences  `json:"recent"`
}

func ApplySuggestionsPreferences(requestQuery querytranslate.RSQuery, preferences *Preferences) (isApplied bool) {
	for i, query := range requestQuery.Query {
		if query.Type == querytranslate.Suggestion {
			if query.SearchBoxId != nil {
				searchbox := uibuilder.GetCachedSearchBox(*query.SearchBoxId)
				if searchbox != nil {
					if searchbox.Enabled == nil || *searchbox.Enabled {
						if searchbox.SearchBox != nil {
							// Apply searchbox preferences
							if searchbox.SearchBox.Popular != nil {
								requestQuery.Query[i].PopularSuggestionsConfig = searchbox.SearchBox.Popular
							}
							if searchbox.SearchBox.Recent != nil {
								requestQuery.Query[i].RecentSuggestionsConfig = searchbox.SearchBox.Recent
							}
							if searchbox.SearchBox.Featured != nil && searchbox.SearchBox.Featured.Layout != nil {
								featuredSuggestionsConfig := requestQuery.Query[i].FeaturedSuggestionsConfig
								if featuredSuggestionsConfig == nil {
									featuredSuggestionsConfig = &querytranslate.FeaturedSuggestionsOptions{}
								}
								if searchbox.SearchBox.Featured.Layout.MaxSuggestionsPerSection != nil {
									featuredSuggestionsConfig.MaxSuggestionsPerSection = searchbox.SearchBox.Featured.Layout.MaxSuggestionsPerSection
								}
								if searchbox.SearchBox.Featured.Layout.SectionsOrder != nil {
									featuredSuggestionsConfig.SectionsOrder = searchbox.SearchBox.Featured.Layout.SectionsOrder
								}
								requestQuery.Query[i].FeaturedSuggestionsConfig = featuredSuggestionsConfig
							}
							if searchbox.SearchBox.Endpoint != nil {
								endpointPreferences := searchbox.SearchBox.Endpoint
								if endpointPreferences.Endpoint != nil && (query.EnableEndpointSuggestions == nil || *query.EnableEndpointSuggestions) {
									var endpoint *querytranslate.Endpoint
									var body *interface{}
									if endpointPreferences.Endpoint.Body != nil {
										var b interface{}
										err := json.Unmarshal([]byte(*endpointPreferences.Endpoint.Body), &b)
										if err != nil {
											log.Errorln(logTag, ":", err)
										}
										body = &b
									}

									var headers *map[string]string
									if endpointPreferences.Endpoint.Headers != nil {
										var h map[string]string
										err := json.Unmarshal([]byte(*endpointPreferences.Endpoint.Headers), &h)
										if err != nil {
											log.Errorln(logTag, ":", err)
										}
										headers = &h
									}
									e := querytranslate.Endpoint{
										URL:     endpointPreferences.Endpoint.URL,
										Method:  endpointPreferences.Endpoint.Method,
										Headers: headers,
										Body:    body,
									}
									endpoint = &e
									requestQuery.Query[i].Endpoint = endpoint
								}
								// Apply endpoint preferences
								if query.ShowDistinctSuggestions == nil {
									requestQuery.Query[i].ShowDistinctSuggestions = endpointPreferences.ShowDistinctSuggestions
								}
								if query.EnablePredictiveSuggestions == nil {
									requestQuery.Query[i].EnablePredictiveSuggestions = endpointPreferences.EnablePredictiveSuggestions
								}

								if query.MaxPredictedWords == nil {
									requestQuery.Query[i].MaxPredictedWords = endpointPreferences.MaxPredictedWords
								}
								if query.ApplyStopwords == nil {
									requestQuery.Query[i].ApplyStopwords = endpointPreferences.ApplyStopwords
								}
								if query.Stopwords == nil {
									requestQuery.Query[i].Stopwords = endpointPreferences.CustomStopwords
								}
								if query.EnableSynonyms == nil {
									requestQuery.Query[i].EnableSynonyms = endpointPreferences.EnableSynonyms
								}
								if query.IncludeFields == nil {
									requestQuery.Query[i].IncludeFields = endpointPreferences.IncludeFields
								}
								if query.ExcludeFields == nil {
									requestQuery.Query[i].ExcludeFields = endpointPreferences.ExcludeFields
								}
								if query.URLField == nil {
									requestQuery.Query[i].URLField = endpointPreferences.URLField
								}
							}
						}
					}
				}
			} else if preferences != nil {
				// Apply index preferences
				indexPreferences := preferences.Index
				if query.Size == nil {
					requestQuery.Query[i].Size = indexPreferences.Size
				}
				if query.Index == nil && len(indexPreferences.Indices) > 0 {
					indexPattern := strings.Join(indexPreferences.Indices, ",")
					// we can apply any pattern (except '*') if present in index suggestions preferences
					if indexPattern != "*" {
						requestQuery.Query[i].Index = &indexPattern
					}
				}
				if query.ShowDistinctSuggestions == nil {
					requestQuery.Query[i].ShowDistinctSuggestions = indexPreferences.ShowDistinctSuggestions
				}

				if query.EnablePredictiveSuggestions == nil {
					requestQuery.Query[i].EnablePredictiveSuggestions = indexPreferences.EnablePredictiveSuggestions
				}

				if query.MaxPredictedWords == nil {
					requestQuery.Query[i].MaxPredictedWords = indexPreferences.MaxPredictedWords
				}

				if query.CategoryField == nil {
					requestQuery.Query[i].CategoryField = indexPreferences.CategoryField
				}

				if query.URLField == nil {
					requestQuery.Query[i].URLField = indexPreferences.URLField
				}

				if indexPreferences.CustomQuery != nil &&
					*indexPreferences.CustomQuery != "" {
					if query.DefaultQuery != nil {
						query := *query.DefaultQuery
						if query["id"] != nil {
							query["id"] = *indexPreferences.CustomQuery
						}
						requestQuery.Query[i].DefaultQuery = &query
					} else {
						var value interface{} = ""
						if query.Value != nil {
							value = *query.Value
						}
						defaultQuery := map[string]interface{}{
							"id": *indexPreferences.CustomQuery,
							"params": map[string]interface{}{
								"query": value,
							},
						}
						requestQuery.Query[i].DefaultQuery = &defaultQuery
					}
				}
				if query.ApplyStopwords == nil {
					requestQuery.Query[i].ApplyStopwords = indexPreferences.ApplyStopwords
				}
				if query.Stopwords == nil {
					requestQuery.Query[i].Stopwords = &indexPreferences.CustomStopwords
				}
				// Note: We apply some of the index suggestions preferences in the search relevancy
				// middleware because searchrelevancy middleware gets executed before the suggestions
				// middleware and the `nil` property check would fail for the properties modified by
				// searchrelevancy middleware.
				recentPreferences := preferences.Recent
				popularPreferences := preferences.Popular
				if query.EnableRecentSuggestions != nil && *query.EnableRecentSuggestions {
					var recentSuggestionConfig = querytranslate.RecentSuggestionsOptions{}
					if query.RecentSuggestionsConfig != nil {
						recentSuggestionConfig = *query.RecentSuggestionsConfig
					}
					if recentSuggestionConfig.Index == nil && len(recentPreferences.Indices) > 0 {
						indexPattern := strings.Join(recentPreferences.Indices, ",")
						// we can apply any pattern (except '*') if present in recent suggestions preferences
						if indexPattern != "*" {
							recentSuggestionConfig.Index = &indexPattern
						}
					}
					if recentSuggestionConfig.MinChars == nil {
						recentSuggestionConfig.MinChars = recentPreferences.MinChars
					}
					if recentSuggestionConfig.MinHits == nil {
						recentSuggestionConfig.MinHits = recentPreferences.MinHits
					}
					if recentSuggestionConfig.Size == nil {
						recentSuggestionConfig.Size = recentPreferences.Size
					}
					requestQuery.Query[i].RecentSuggestionsConfig = &recentSuggestionConfig
				}

				if query.EnablePopularSuggestions != nil && *query.EnablePopularSuggestions {
					var popularPreferencesConfig = querytranslate.PopularSuggestionsOptions{}
					if query.PopularSuggestionsConfig != nil {
						popularPreferencesConfig = *query.PopularSuggestionsConfig
					}
					if popularPreferencesConfig.Index == nil && len(popularPreferences.Indices) > 0 {
						indexPattern := strings.Join(popularPreferences.Indices, ",")
						// we can apply any pattern (including '*') if present in popular suggestions preferences
						popularPreferencesConfig.Index = &indexPattern
					}
					if popularPreferencesConfig.Size == nil {
						popularPreferencesConfig.Size = &popularPreferences.Size
					}
					requestQuery.Query[i].PopularSuggestionsConfig = &popularPreferencesConfig
				}
			}
			isApplied = true
		}
	}
	return
}

func ApplySuggestions(
	requestQuery querytranslate.RSQuery,
	originalResponse []byte,
	recentSuggestionsMap map[string]SuggestionOutput,
	popularSuggestionsMap map[string]SuggestionOutput,
	featuredSuggestionsMap map[string]SuggestionOutput,
	faqSuggestionsMap map[string]SuggestionOutput,
	documentSuggestionsMap map[string]SuggestionOutput,
) ([]byte, *Error) {
	response := originalResponse
	for _, query := range requestQuery.Query {
		if query.Type == querytranslate.Suggestion {
			// suggestions config config object
			suggestionsConfig := querytranslate.SuggestionsConfig{
				ApplyStopwords: query.ApplyStopwords,
				Stopwords:      query.Stopwords,
				Language:       query.SearchLanguage,
			}
			// to maintain the uniqueness of suggestions
			suggestionValue := make(map[string]bool)
			finalSuggestions := []querytranslate.SuggestionHIT{}
			// list of popular suggestions
			popularSuggestions := []querytranslate.SuggestionHIT{}
			// list of recent suggestions
			recentSuggestions := []querytranslate.SuggestionHIT{}
			// list of featured suggestions
			featuredSuggestions := []querytranslate.SuggestionHIT{}

			// list of faq suggestions
			faqSuggestions := []querytranslate.SuggestionHIT{}

			// list of document suggestions
			documentSuggestions := []querytranslate.SuggestionHIT{}

			// only apply popular suggestions
			if query.EnablePopularSuggestions != nil &&
				*query.EnablePopularSuggestions {
				if out, ok := popularSuggestionsMap[*query.ID]; ok {
					if out.Error != nil {
						return response, out.Error
					} else {
						popularSuggestions = out.Suggestions
					}
				}
			}
			// apply recent suggestions
			if query.EnableRecentSuggestions != nil &&
				*query.EnableRecentSuggestions {
				if out, ok := recentSuggestionsMap[*query.ID]; ok {
					if out.Error != nil {
						return response, out.Error
					} else {
						recentSuggestions = out.Suggestions
					}
				}
			}

			// apply featured suggestions
			if out, ok := featuredSuggestionsMap[*query.ID]; ok {
				if out.Error != nil {
					return response, out.Error
				} else {
					featuredSuggestions = out.Suggestions
				}
			}

			if query.EnableFAQSuggestions != nil && *query.EnableFAQSuggestions {
				// apply faq suggestions
				if out, ok := faqSuggestionsMap[*query.ID]; ok {
					if out.Error != nil {
						return response, out.Error
					} else {
						faqSuggestions = out.Suggestions
					}
				}
			}

			if query.EnableDocumentSuggestions != nil && *query.EnableDocumentSuggestions {
				// apply document suggestions
				if out, ok := documentSuggestionsMap[*query.ID]; ok {
					if out.Error != nil {
						return response, out.Error
					} else {
						documentSuggestions = out.Suggestions
					}
				}
			}

			// append recent suggestions at top
			// append popular suggestions after recent suggestions
			// move index suggestions to the end
			// Note: Promoted suggestions would always be at top
			if len(popularSuggestions) > 0 || len(recentSuggestions) > 0 || len(featuredSuggestions) > 0 || len(faqSuggestions) > 0 || len(documentSuggestions) > 0 {
				// apply faq suggestions
				for _, suggestion := range faqSuggestions {
					if !suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] {
						finalSuggestions = append(finalSuggestions, suggestion)
						suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] = true
					}
				}

				// apply featured suggestions
				for _, suggestion := range featuredSuggestions {
					if !suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] {
						finalSuggestions = append(finalSuggestions, suggestion)
						suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] = true
					}
				}
				// apply recent suggestions
				for _, suggestion := range recentSuggestions {
					if !suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] {
						finalSuggestions = append(finalSuggestions, suggestion)
						suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] = true
					}
				}
				// apply popular suggestions
				for _, suggestion := range popularSuggestions {
					if !suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] {
						finalSuggestions = append(finalSuggestions, suggestion)
						suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] = true
					}
				}

				// apply document suggestions
				for _, suggestion := range documentSuggestions {
					if !suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] {
						finalSuggestions = append(finalSuggestions, suggestion)
						suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] = true
					}
				}
			}
			var indexSuggestions []querytranslate.SuggestionHIT

			var transformResponse *string
			var searchbox *uibuilder.SearchBoxESModel
			// transform response
			if query.SearchBoxId != nil {
				searchbox = uibuilder.GetCachedSearchBox(*query.SearchBoxId)
				if searchbox != nil &&
					(searchbox.Enabled == nil || *searchbox.Enabled) &&
					searchbox.SearchBox != nil &&
					searchbox.SearchBox.Endpoint != nil &&
					((query.EnableEndpointSuggestions != nil && *query.EnableEndpointSuggestions) ||
						(searchbox.SearchBox.Featured.Design != nil &&
							(*searchbox.SearchBox.Featured.Design)["enableEndpointSuggestions"].(bool) == true)) &&
					searchbox.SearchBox.Endpoint.TransformResponse != nil {
					transformResponse = searchbox.SearchBox.Endpoint.TransformResponse
				}
			}
			if transformResponse != nil {
				// since we're applying popular and recent suggestions, re-parse index suggestions and
				// append at the end
				indexSuggestionsBytes, valueType, _, err := jsonparser.Get(response, *query.ID)
				if valueType == jsonparser.NotExist {
					continue
				}
				if err != nil {
					log.Errorln(logTag, ":", err)
					return response, &Error{Error: err}
				}
				searchboxInBytes, _ := json.Marshal(searchbox)
				// translate mongodb query response to RS API response
				script := fmt.Sprintf(`function handleRequest() {
			const a = eval(%s);
			return a(%s, %s);
		}`, *transformResponse, string(indexSuggestionsBytes), string(searchboxInBytes))
				scriptOutput, _, err := rules.RunScript(nil, script, 5*time.Second)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return response, &Error{Error: err}
				}
				err2 := json.Unmarshal(scriptOutput, &indexSuggestions)
				if err2 != nil {
					log.Errorln(logTag, ":", err2)
					return response, &Error{Error: err2}
				}
				for i := range indexSuggestions {
					// use suggestion type as endpoint
					indexSuggestions[i].Type = querytranslate.EndpointSuggestion
				}
			} else {
				// since we're applying popular and recent suggestions, re-parse index suggestions and
				// append at the end
				indexSuggestionsBytes, valueType, _, err := jsonparser.Get(response, *query.ID, "hits", "hits")
				if valueType == jsonparser.NotExist {
					continue
				}
				if err != nil {
					log.Errorln(logTag, ":", err)
					return response, &Error{Error: err}
				}
				err2 := json.Unmarshal(indexSuggestionsBytes, &indexSuggestions)
				if err2 != nil {
					log.Errorln(logTag, ":", err2)
					return response, &Error{Error: err2}
				}
			}

			// populate index suggestions
			for _, suggestion := range indexSuggestions {
				if !suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] {
					finalSuggestions = append(finalSuggestions, suggestion)
					suggestionValue[querytranslate.CompressAndOrder(suggestion.Value, suggestionsConfig)] = true
				}
			}
			// trim suggestions as per size
			if query.Size != nil && len(finalSuggestions) > *query.Size {
				finalSuggestions = finalSuggestions[:*query.Size]
			}
			finalSuggestionsBytes, err3 := json.Marshal(finalSuggestions)
			if err3 != nil {
				log.Errorln(logTag, ":", err3)
				return response, &Error{Error: err3}
			}
			var err4 error
			response, err4 = jsonparser.Set(response, finalSuggestionsBytes, *query.ID, "hits", "hits")
			if err4 != nil {
				log.Errorln(logTag, ":", err4)
				return response, &Error{Error: err4}
			}
			// Modify total suggestions value
			var err5 error
			response, err5 = jsonparser.Set(response, []byte(strconv.Itoa(len(finalSuggestions))), *query.ID, "hits", "total", "value")
			if err5 != nil {
				log.Errorln(logTag, ":", err5)
				return response, &Error{Error: err5}
			}
		}
	}
	return response, nil
}
