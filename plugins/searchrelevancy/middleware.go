package searchrelevancy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/ratelimiter"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/difference"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/request"
	"github.com/appbaseio/reactivesearch-api/model/requestlogs"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/cache"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/suggestions"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

// A list of plans for which search relevancy will be enabled
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
	util.HostedArcStandard2021,
	util.Starter2021,
	util.Sandbox2023,
	util.Starter2023,
	util.ProductionFirst2023,
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classify.Op(),
		classify.Indices(),
		logs.Recorder(),
		auth.BasicAuth(),
		validate.Sources(),
		ratelimiter.Limit(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
		validate.Plan(validPlans, util.GetFeatureSearchRelevancy(), "Search Relevancy feature"),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.SearchRelevancy

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func (s *SearchRelevancy) intercept(h http.HandlerFunc) http.HandlerFunc {
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

		requestQuery, err := querytranslate.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		// don't apply search relevancy if disabled
		if requestQuery.Settings != nil &&
			requestQuery.Settings.EnableSearchRelevancy != nil &&
			!*requestQuery.Settings.EnableSearchRelevancy {
			h(w, req)
			return
		}

		start := time.Now()
		var shouldCalculateDiff = true

		// Extract the disable-logs flag value
		isLogsDisabled, errFetching := difference.FromContext(req.Context())
		if errFetching != nil {
			log.Warnln(logTag, ": error while fetching value of `disable-logs`: ", errFetching.Error())
			defaultLogsDisabled := false
			isLogsDisabled = &defaultLogsDisabled
		}

		requestInfo, err := request.FromRequestIDContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request-id from context", http.StatusInternalServerError)
			return
		}

		stage := "searchrelevancy"
		if requestInfo != nil && !*isLogsDisabled {
			// Records logs
			rl := requestlogs.Get(requestInfo.Id)
			if rl != nil {
				rl.LogsDiffing.Add(1)
				go func(request *querytranslate.RSQuery, out chan<- requestlogs.LogsResults) {
					// Marshal the body and save it as the one before modification
					marshalledReqBody, err := json.Marshal(request)
					if err != nil {
						log.Warnln(logTag, "error while marshalling request query, ", err)
						shouldCalculateDiff = false
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

		var searchRelevancySettings SearchRelevancyStruct
		// get the searchRelevancySettings cache
		var cachedRelevancySettings = GetSearchRelevancySettingsFromCache()
		// currently available indexes in cache
		var cachedIndexes = []string{}
		for k := range cachedRelevancySettings {
			cachedIndexes = append(cachedIndexes, k)
		}

		// appliedRelevancySetting is string to return in response. It holds the name of index whose setting is applied in case of multiple indices being passed in url
		appliedRelevancySetting := []byte("")
		indices, err := index.FromContext(req.Context())
		for _, index := range indices {
			// '*' in case of all indices select first search setting from cache
			if index == "*" {
				if len(cachedIndexes) > 0 {
					appliedRelevancySetting = []byte(cachedIndexes[0])
					searchRelevancySettings, err = GetSearchRelevancySettingFromCache(cachedIndexes[0])
				}
				break
			} else if strings.Contains(index, "*") {
				// in case of regex check if string contains '*' in naming pattern, if contains and doesn't have '.*' and replace '*' with '.*' because golang regex can match in that pattern. Next match regex patters with existing index names in cache and find first matching pattern.
				regex := index

				if !strings.Contains(index, ".*") {
					regex = strings.Replace(regex, "*", ".*", -1)
				}
				r, _ := regexp.Compile(regex)

				for _, val := range cachedIndexes {
					if r.MatchString(val) {
						appliedRelevancySetting = []byte(val)
						searchRelevancySettings, err = GetSearchRelevancySettingFromCache(val)
						break
					}
				}

			} else {
				// if multiple indexes are passed select first index and apply searchRelevancySettings accordingly
				searchRelevancySettings, err = GetSearchRelevancySettingFromCache(index)
				if err == nil {
					appliedRelevancySetting = []byte(index)
					break
				}
			}
		}
		if string(appliedRelevancySetting) == "" {
			searchRelevancySettings = getDefaultRelevancySettings()
		}

		for i, query := range requestQuery.Query {
			// apply search language
			if query.SearchLanguage == nil {
				if searchRelevancySettings.Language != nil &&
					searchRelevancySettings.Language.Language != "" {
					language := searchRelevancySettings.Language.Language
					requestQuery.Query[i].SearchLanguage = &language
				}
			}
			// search
			if query.Type == querytranslate.Search || query.Type == querytranslate.Suggestion {
				// Apply rank feature to all kind of search queries
				if query.RankFeature == nil {
					requestQuery.Query[i].RankFeature = searchRelevancySettings.Search.RankFeature
				}
				if query.Value != nil {
					if query.Type == querytranslate.Suggestion {
						// Note: We apply some of the index suggestions preferences in the search relevancy
						// middleware because searchrelevancy middleware gets executed before the suggestions
						// middleware and the `nil` property check would fail for the properties modified by
						// searchrelevancy middleware.
						indexPreferences := suggestions.GetIndexPreferences()
						if query.Size == nil &&
							indexPreferences.Size != nil {
							requestQuery.Query[i].Size = indexPreferences.Size
						}

						if query.EnableSynonyms == nil &&
							indexPreferences.EnableSynonyms != nil {
							requestQuery.Query[i].EnableSynonyms = indexPreferences.EnableSynonyms
						}

						if query.IncludeFields == nil &&
							indexPreferences.IncludeFields != nil {
							requestQuery.Query[i].IncludeFields = &indexPreferences.IncludeFields
						}

						if query.ExcludeFields == nil &&
							indexPreferences.ExcludeFields != nil {
							requestQuery.Query[i].ExcludeFields = &indexPreferences.ExcludeFields
						}
					}
					if query.SearchOperators == nil {
						requestQuery.Query[i].SearchOperators = &searchRelevancySettings.Search.SearchOperators
					}

					if query.QueryString == nil {
						requestQuery.Query[i].QueryString = &searchRelevancySettings.Search.QueryString
					}

					if query.Fuzziness == nil {
						requestQuery.Query[i].Fuzziness = searchRelevancySettings.Search.Fuzziness
					}
					normalizedFields := querytranslate.NormalizedDataFields(query.DataField, query.FieldWeights)
					if len(normalizedFields) == 0 && len(searchRelevancySettings.Search.DataField) > 0 {
						requestQuery.Query[i].DataField = searchRelevancySettings.Search.DataField
						fieldWeights := []float64{}
						for _, v := range searchRelevancySettings.Search.FieldWeights {
							strVal := fmt.Sprintf("%v", v)
							floatVal, _ := strconv.ParseFloat(strVal, 64)
							fieldWeights = append(fieldWeights, floatVal)
						}
						if len(searchRelevancySettings.Search.FieldWeights) > 0 {
							requestQuery.Query[i].FieldWeights = fieldWeights
						}
					}

					if query.EnableSynonyms == nil {
						requestQuery.Query[i].EnableSynonyms = &searchRelevancySettings.Synonyms.Enabled
					}

					// queryFormat is nil and searchRelevancy setting is present then use that as default queryFormat in search query
					if query.QueryFormat == nil && searchRelevancySettings.Search.QueryFormat != nil {
						requestQuery.Query[i].QueryFormat = searchRelevancySettings.Search.QueryFormat
					}
				}
				// Apply distinctField
				if query.DistinctField == nil && searchRelevancySettings.Search.DistinctField != "" {
					requestQuery.Query[i].DistinctField = &searchRelevancySettings.Search.DistinctField
				}
				// Result Settings
				// Apply highlight settings
				if query.Highlight == nil {
					requestQuery.Query[i].Highlight = &searchRelevancySettings.Results.Highlight
				}

				if *requestQuery.Query[i].Highlight {
					if len(query.HighlightField) == 0 && len(searchRelevancySettings.Results.HighlightFields) > 0 {
						requestQuery.Query[i].HighlightField = searchRelevancySettings.Results.HighlightFields
					}

					if query.CustomHighlight == nil &&
						query.HighlightConfig == nil &&
						searchRelevancySettings.Results.Highlight {
						// check if fields is present in highlightOptions
						_, ok := searchRelevancySettings.Results.HighlightOptions["fields"]
						if !ok {
							// if the fields is not present add the fields that are set in HighlightField
							highlightFields := make(map[string]interface{})

							for _, field := range requestQuery.Query[i].HighlightField {
								highlightFields[field] = make(map[string]interface{})
							}
							searchRelevancySettings.Results.HighlightOptions["fields"] = highlightFields
						}

						if len(searchRelevancySettings.Results.HighlightFields) > 0 {
							requestQuery.Query[i].HighlightConfig = &searchRelevancySettings.Results.HighlightOptions
						}
					}
				}
				// Only apply size for search type of queries
				if query.Size == nil {
					requestQuery.Query[i].Size = &searchRelevancySettings.Results.Size
				}
			}

			if query.IncludeFields == nil {
				requestQuery.Query[i].IncludeFields = &searchRelevancySettings.Results.IncludeFields
			}

			if query.ExcludeFields == nil {
				requestQuery.Query[i].ExcludeFields = &searchRelevancySettings.Results.ExcludeFields
			}

			// Term Aggregation
			if query.Type == querytranslate.Term {
				if query.AggregationSize == nil {
					requestQuery.Query[i].AggregationSize = &searchRelevancySettings.Aggregations.Size
				}
				if query.SortBy == nil {
					requestQuery.Query[i].SortBy = searchRelevancySettings.Aggregations.SortBy
				}

				// queryFormat is nil and searchRelevancy setting is present then use that as default queryFormat in aggregation queries
				if query.QueryFormat == nil && searchRelevancySettings.Aggregations.QueryFormat != nil {
					requestQuery.Query[i].QueryFormat = searchRelevancySettings.Aggregations.QueryFormat
				}
			}

			// Range Aggregation
			if query.Type == querytranslate.Range {
				if query.IncludeNullValues == nil {
					requestQuery.Query[i].IncludeNullValues = &searchRelevancySettings.Aggregations.IncludeNullValues
				}
			}

		}

		if shouldCalculateDiff {
			if requestInfo != nil && !*isLogsDisabled {
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

		rsAPIctx := querytranslate.NewContext(req.Context(), *requestQuery)
		req = req.WithContext(rsAPIctx)

		// Track plugin
		ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "sr")
		req = req.WithContext(ctxTrackPlugin)

		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, req)
		// copy the response to writer
		for k, v := range resp.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.Code)

		// Avoid response modification for cached responses
		if resp.Header().Get(cache.CachedRequestHeader) == "true" {
			w.Write(resp.Body.Bytes())
			return
		}

		// Avoid writing settings for _reactivesearch.v3/validate route
		if !util.IsRSAPIValidateRoute(req) && string(appliedRelevancySetting) != "" {
			responseBody, err := jsonparser.Set(resp.Body.Bytes(), []byte(fmt.Sprintf("%q", appliedRelevancySetting)), "settings", "searchRelevancy")
			if err != nil {
				log.Warnln(logTag, "unable to set searchRelevancy key in settings", err)
				responseBody2, err2 := jsonparser.Set(resp.Body.Bytes(), []byte(fmt.Sprintf(`{ "searchRelevancy": %q }`, appliedRelevancySetting)), "settings")
				if err2 != nil {
					log.Warnln(logTag, "unable to set settings key", err2)
				} else {
					w.Write(responseBody2)
					return
				}
			} else {
				w.Write(responseBody)
				return
			}
		}
		w.Write(resp.Body.Bytes())
	}
}
