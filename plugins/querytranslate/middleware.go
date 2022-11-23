package querytranslate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/gorilla/mux"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/ratelimiter"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/request"
	"github.com/appbaseio/reactivesearch-api/model/requestlogs"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(mw []middleware.Middleware, h http.HandlerFunc) http.HandlerFunc {
	// Append query translate middleware at the end
	mw = append(mw, queryTranslate)
	// Append telemetry at the end
	mw = append(mw, telemetry.Recorder())
	return c.Adapt(h, append(list(), mw...)...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		validate.Elasticsearch(),
		classifyCategory,
		classifyOp,
		classify.Indices(),
		saveRequestToCtx, // middleware to save the request body in context
		logs.Recorder(),
		auth.BasicAuth(),
		ratelimiter.Limit(),
		validate.Sources(),
		validate.Referers(),
		validate.Indices(),
		validate.Category(),
		validate.Operation(),
		validate.PermissionExpiry(),
		applySourceFiltering,
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.ReactiveSearch

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// search requests are are a read operation
		operation := op.Read
		ctx := op.NewContext(req.Context(), &operation)
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func saveRequestToCtx(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body RSQuery
		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf("Can't parse request body: %v", err), http.StatusBadRequest)
			return
		}

		err = json.Unmarshal(buf.Bytes(), &body)
		if err != nil {
			log.Errorln(logTag, "error while unmarshalling request body to save to context", err)
			return
		}

		// Replace original body with the same body
		// since it was emptied when we read it.
		req.Body = ioutil.NopCloser(buf)

		// No need to write original body, it will be written by search relevancy
		// since the first modification happens there.
		// NOTE: Set the original request if searchrelevancy is not available, i:e oss
		originalReq, err := request.FromContext(req.Context())
		if *originalReq == nil {
			log.Warnln(logTag, "Setting original request body since nil was found: ", *originalReq)
			originalCtx := request.NewContext(req.Context(), body)
			req = req.WithContext(originalCtx)
		}
		// Forward context with request Id
		ctx := request.NewRequestIDContext(NewContext(req.Context(), body), buf.Bytes())
		req = req.WithContext(ctx)
		requestInfo, err := request.FromRequestIDContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request-id from context", http.StatusInternalServerError)
			return
		}
		if requestInfo != nil {
			var wg sync.WaitGroup
			// Initialize logger
			requestlogs.Put(requestInfo.Id, requestlogs.ActiveRequestLog{
				LogsDiffing: &wg,
				Output:      make(chan requestlogs.LogsResults),
			})
		}
		h(w, req)
	}
}

func applySourceFiltering(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		// Apply source filters
		reqPermission, err := permission.FromContext(ctx)
		if err != nil {
			log.Warnln(logTag, ":", err)
			h(w, req)
			return
		}
		isExcludesPresent := len(reqPermission.Excludes) != 0
		isEmpty := len(reqPermission.Includes) == 0 && len(reqPermission.Excludes) == 0
		isDefaultInclude := len(reqPermission.Includes) > 0 && reqPermission.Includes[0] == "*"
		shouldApplyFilters := !isEmpty && (!isDefaultInclude || isExcludesPresent)
		if shouldApplyFilters {
			requestQuery, err := FromContext(req.Context())
			if err != nil {
				log.Errorln(logTag, ":", err)

				telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
				return
			}
			for index := range requestQuery.Query {
				requestQuery.Query[index].IncludeFields = &reqPermission.Includes
				requestQuery.Query[index].ExcludeFields = &reqPermission.Excludes
			}
		}
		h(w, req)
	}
}

// Translates the query to `_msearch` request
func queryTranslate(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()

		// Extract the index from the vars
		vars := mux.Vars(req)

		shouldLogDiff := true
		stage := "querytranslate"

		body, err := FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		requestInfo, err := request.FromRequestIDContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request-id from context", http.StatusInternalServerError)
			return
		}

		// Records logs
		rl := requestlogs.Get(requestInfo.Id)
		if rl != nil {
			rl.LogsDiffing.Add(1)
			go func(body *RSQuery, out chan<- requestlogs.LogsResults) {
				// Marshal the body and save it as the one before modification
				marshalledBody, err := json.Marshal(body)
				if err != nil {
					log.Warnln(logTag, "couldn't marshal body to run log diff on it, ", err)
					shouldLogDiff = false
				}
				defer rl.LogsDiffing.Done()
				// Write log output
				out <- requestlogs.LogsResults{
					LogType: "request",
					LogTime: "before",
					Data: requestlogs.RequestData{
						Body:    string(marshalledBody),
						Method:  req.Method,
						Headers: req.Header,
						URL:     req.URL.Path,
					},
					Stage: stage,
				}
			}(body, rl.Output)
		}

		// validate request by permission
		reqPermission, err := permission.FromContext(req.Context())
		if err != nil {
			log.Warnln(logTag, ":", err)
		}
		if reqPermission != nil && reqPermission.ReactiveSearchConfig != nil {
			for _, query := range body.Query {
				// Note: Query DSL validation is handled by noss
				// validate query size
				if reqPermission.ReactiveSearchConfig.MaxSize != nil {
					// validate size from defaultQuery if present
					if query.DefaultQuery != nil {
						defaultQuery := *query.DefaultQuery
						if defaultQuery["size"] != nil {
							sizeAsFloat, ok := defaultQuery["size"].(float64)
							if ok {
								intSize := int(sizeAsFloat)
								querySize := &intSize
								// throw error if query size is greater than specified size
								if querySize != nil && *querySize > *reqPermission.ReactiveSearchConfig.MaxSize {
									errorMsg := "maximum allowed size is " + strconv.Itoa(*reqPermission.ReactiveSearchConfig.MaxSize)
									telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
									return
								}
							}
						}
					}

					if query.Size != nil {
						querySize := query.Size
						// throw error if query size is greater than specified size
						if querySize != nil && *querySize > *reqPermission.ReactiveSearchConfig.MaxSize {
							errorMsg := "maximum allowed size is " + strconv.Itoa(*reqPermission.ReactiveSearchConfig.MaxSize)
							telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
							return
						}
					}
				}

				// validate aggregation size
				if reqPermission.ReactiveSearchConfig.MaxAggregationSize != nil {
					// validate size from defaultQuery if present
					if query.DefaultQuery != nil {
						size := getSizeFromQuery(query.DefaultQuery, "size")
						if size != nil {
							sizeAsFloat, ok := (*size).(float64)
							if ok {
								sizeAsInt := int(sizeAsFloat)
								aggsSize := &sizeAsInt
								// throw error if query size is greater than specified size
								if aggsSize != nil && *aggsSize > *reqPermission.ReactiveSearchConfig.MaxAggregationSize {
									errorMsg := "maximum allowed aggregation size is " + strconv.Itoa(*reqPermission.ReactiveSearchConfig.MaxAggregationSize)
									telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
									return
								}
							}
						}
					}
					if query.AggregationSize != nil {
						aggsSize := query.AggregationSize
						// throw error if query size is greater than specified size
						if aggsSize != nil && *aggsSize > *reqPermission.ReactiveSearchConfig.MaxAggregationSize {
							errorMsg := "maximum allowed aggregation size is " + strconv.Itoa(*reqPermission.ReactiveSearchConfig.MaxAggregationSize)
							telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
							return
						}
					}
				}
			}
		}

		for i, query := range body.Query {
			// apply default highlight for suggestions
			if query.Type == Suggestion &&
				query.Highlight != nil && *query.Highlight &&
				(query.HighlightConfig == nil && query.CustomHighlight == nil) {
				defaultHighlight := getDefaultSuggestionsHighlight(query)
				body.Query[i].HighlightConfig = &defaultHighlight
			}
		}

		// Translate query
		var translateErr error
		var msearchQuery string

		var preference *string
		p := req.URL.Query().Get("preference")
		if p != "" {
			preference = &p
		}
		msearchQuery, _, translateErr = translateQuery(*body, iplookup.FromRequest(req), nil, preference)

		// log.Println("RS QUERY", msearchQuery)
		if translateErr != nil {
			log.Errorln(logTag, ":", translateErr)
			telemetry.WriteBackErrorWithTelemetry(req, w, translateErr.Error(), http.StatusBadRequest)
			return
		}

		// Update the request body to the parsed query
		req.Body = ioutil.NopCloser(strings.NewReader(msearchQuery))

		// Build independent queries as well
		independentRequests, independentReqErr := buildIndependentRequests(*body)
		if independentReqErr != nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, independentReqErr.Error(), http.StatusBadRequest)
			return
		}

		// Inject the independent requests
		updatedCtx := NewIndependentRequestContext(req.Context(), independentRequests)
		req = req.WithContext(updatedCtx)

		if shouldLogDiff {
			if rl != nil {
				rl.LogsDiffing.Add(1)
				timeTaken := float64(time.Since(start).Milliseconds())
				go func(body string, timeTaken float64, out chan<- requestlogs.LogsResults) {
					defer rl.LogsDiffing.Done()
					// Diff the URI manually for this stage
					esURL := "/" + vars["index"] + "/_msearch"
					// Write log output
					out <- requestlogs.LogsResults{
						LogType: "request",
						LogTime: "after",
						Data: requestlogs.RequestData{
							Body:    msearchQuery,
							Method:  req.Method,
							Headers: req.Header,
							URL:     esURL,
						},
						Stage:     stage,
						TimeTaken: timeTaken,
					}
				}(msearchQuery, timeTaken, rl.Output)
			}
		}

		// Track plugin
		ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "qt")
		req = req.WithContext(ctxTrackPlugin)

		h(w, req)
	}
}
