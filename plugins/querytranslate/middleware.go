package querytranslate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/gorilla/mux"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/ratelimiter"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/difference"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/request"
	"github.com/appbaseio/reactivesearch-api/model/requestchange"
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
		start := time.Now()

		// Extract the index from the vars
		vars := mux.Vars(req)

		shouldLogDiff := true

		body, err := FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		// Marshal the body and save it as the one before modification
		marshalledBody, err := json.Marshal(body)
		if err != nil {
			log.Warnln(logTag, "couldn't marshal body to run log diff on it, ", err)
			shouldLogDiff = false
		}

		reqBodyBeforeModification := ioutil.NopCloser(strings.NewReader(string(marshalledBody)))

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

		msearchQuery, translateErr = translateQuery(*body, iplookup.FromRequest(req))

		// log.Println("RS QUERY", msearchQuery)
		if translateErr != nil {
			log.Errorln(logTag, ":", translateErr)
			telemetry.WriteBackErrorWithTelemetry(req, w, translateErr.Error(), http.StatusBadRequest)
			return
		}

		// Update the request body to the parsed query
		req.Body = ioutil.NopCloser(strings.NewReader(msearchQuery))

		if shouldLogDiff {
			reqBodyAfterModification := ioutil.NopCloser(strings.NewReader(string(msearchQuery)))

			bodyDiffStr := util.CalculateBodyDiff(reqBodyBeforeModification, reqBodyAfterModification)

			// Diff the URI manually for this stage
			esURL := "/" + vars["index"] + "/_msearch"

			DiffCalculated := &difference.Difference{
				Body:    bodyDiffStr,
				Headers: util.CalculateHeaderDiff(req.Header, req.Header),
				URI:     util.CalculateStringDiff(req.URL.Path, esURL),
				Method:  util.CalculateMethodDiff(req, req),
			}

			timeTaken := float64(time.Since(start).Milliseconds())
			DiffCalculated.Took = &timeTaken
			DiffCalculated.Stage = "querytranslate"

			// Save the diff to context
			// Get all the diffs first, then append and update the context
			currentDiffs, err := requestchange.FromContext(req.Context())
			if err != nil {
				log.Warnln(logTag, ": error while getting diff, creating new diff. Err: ", err)
				madeDiffs := make([]difference.Difference, 0)
				currentDiffs = &madeDiffs
			}
			*currentDiffs = append(*currentDiffs, *DiffCalculated)

			// Save the value to the context
			diffCtx := requestchange.NewContext(req.Context(), currentDiffs)
			req = req.WithContext(diffCtx)
		}

		// Track plugin
		ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "qt")
		req = req.WithContext(ctxTrackPlugin)

		h(w, req)
	}
}
