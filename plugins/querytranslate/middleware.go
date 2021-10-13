package querytranslate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/ratelimiter"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
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
		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf("Can't parse request body: %v", err), http.StatusBadRequest)
			return
		}
		// Set request body as nil to avoid memory issues (storage duplication)
		req.Body = nil
		ctx := NewContext(req.Context(), body)
		req = req.WithContext(ctx)
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
		body, err := FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
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
				query.CustomHighlight == nil {
				defaultHighlight := getDefaultSuggestionsHighlight(query)
				body.Query[i].CustomHighlight = &defaultHighlight
			}
		}

		// Translate query
		msearchQuery, err := translateQuery(*body)
		// log.Println("RS QUERY", msearchQuery)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		// Update the request body to the parsed query
		req.Body = ioutil.NopCloser(strings.NewReader(msearchQuery))

		// Track plugin
		ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "qt")
		req = req.WithContext(ctxTrackPlugin)

		h(w, req)
	}
}
