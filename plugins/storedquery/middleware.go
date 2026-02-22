package storedquery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
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
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		storedQueryCategory := category.StoredQuery

		ctx := category.NewContext(req.Context(), &storedQueryCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := index.NewContext(req.Context(), []string{defaultStoredQueryEsIndex})
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func (s *StoredQuery) intercept(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		requestQuery, err := querytranslate.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		ctxIndices, err := index.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ": cannot fetch indices from request context,", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving indices from context", http.StatusInternalServerError)
			return
		}
		// validate request by permission
		reqPermission, err := permission.FromContext(req.Context())
		if err != nil {
			log.Warnln(logTag, ":", err)
		}
		// In case of `_reactivesearch/validate` route
		// only parse the stored query when credential has access to storedquery
		if util.IsRSAPIValidateRoute(req) {
			isValid := false
			if reqPermission != nil {
				if reqPermission.HasCategory(category.StoredQuery) {
					isValid = true
				}
			} else {
				reqUser, err := user.FromContext(req.Context())
				if err != nil {
					log.Warnln(logTag, ":", err)
				}
				if reqUser != nil {
					if reqUser.HasCategory(category.StoredQuery) {
						isValid = true
					}
				}
			}
			if !isValid {
				msg := "credential does not have access to Stored Queries category"
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusBadRequest)
				return
			}
		}

		mainIndex := strings.Join(ctxIndices, ",")

		storedQueries := getStoredQueries(*requestQuery)
		// parse stored query
		for i, query := range requestQuery.Query {
			if reqPermission != nil && reqPermission.ReactiveSearchConfig != nil {
				// validate query DSL
				if reqPermission.ReactiveSearchConfig.DisbaleQueryDSL != nil {
					if *reqPermission.ReactiveSearchConfig.DisbaleQueryDSL {
						errorMsg := "raw query DSL is disabled. Please use a stored query instead"
						if query.DefaultQuery != nil {
							defaultQuery := *query.DefaultQuery
							if defaultQuery != nil {
								storedQueryId := defaultQuery["id"]
								if storedQueryId == nil {
									telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
									return
								}
								idAsString, ok := storedQueryId.(string)
								if !ok || idAsString == "" {
									telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
									return
								}
							}
						}
						if query.CustomQuery != nil {
							customQuery := *query.CustomQuery
							if customQuery != nil {
								storedQueryId := customQuery["id"]
								if storedQueryId == nil {
									telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
									return
								}
								idAsString, ok := storedQueryId.(string)
								if !ok || idAsString == "" {
									telemetry.WriteBackErrorWithTelemetry(req, w, errorMsg, http.StatusBadRequest)
									return
								}
							}
						}
					}
				}
			}
			// parse default query
			if query.DefaultQuery != nil {
				index := mainIndex
				if query.Index != nil {
					index = *query.Index
				}
				query, err := getParsedQuery(index, *query.DefaultQuery)
				if err != nil {
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
					return
				}
				if query != nil {
					requestQuery.Query[i].DefaultQuery = query
				}
			}
			// parse custom query
			if query.CustomQuery != nil {
				// Validate index for the queries which would consume customQuery
				for _, q2 := range requestQuery.Query {
					if q2.Index != nil && q2.React != nil {
						if util.Contains(flattenReact(*q2.React), *query.ID) {
							// validate index
							_, err := getParsedQuery(*q2.Index, *query.CustomQuery)
							if err != nil {
								telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
								return
							}
						}
					}
				}
				query, err := getParsedQuery("", *query.CustomQuery)
				if err != nil {
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
					return
				}
				if query != nil {
					requestQuery.Query[i].CustomQuery = query
				}
			}
		}
		// Update the RS API request in context
		rsAPIctx := querytranslate.NewContext(req.Context(), *requestQuery)
		req = req.WithContext(rsAPIctx)
		// Update context with stored queries
		ctxStoredQueries := NewContext(req.Context(), storedQueries)
		req = req.WithContext(ctxStoredQueries)
		// Track plugin
		ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "sq")
		req = req.WithContext(ctxTrackPlugin)
		h(w, req)
	}
}

func validateIndex(index string, pattern string) (bool, error) {
	matched, err := util.ValidateIndex(pattern, index)
	if err != nil {
		log.Errorln("invalid index regexp", pattern, "encountered: ", err)
		return false, err
	}
	if matched {
		return matched, nil
	}
	return false, nil
}

func getParsedQuery(index string, query map[string]interface{}) (*map[string]interface{}, error) {
	id := query["id"]
	params := query["params"]
	if id != nil && id != "" {
		idAsString, ok := id.(string)
		if !ok {
			return nil, nil
		}
		storedQuery := GetStoredQueryFromCache(idAsString)
		if storedQuery == nil {
			return nil, errors.New("invalid stored query id used: " + idAsString)
		}
		if index != "" {
			matched, _ := validateIndex(index, *storedQuery.Index)
			if !matched {
				return nil, errors.New("stored query id " + idAsString + " is not applicable on requested index")
			}
		}
		var paramsAsMap = make(map[string]interface{})
		// avoid throwing error if params are not defined
		if params != nil {
			paramsAsMap, ok = params.(map[string]interface{})
			if !ok {
				return nil, errors.New("params must be an object")
			}
		}

		// parse query
		params := getParams(storedQuery.Params, &paramsAsMap)
		// parse query using params
		parsedQuery, err := renderQuery(storedQuery.QueryInBytes, params)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, err
		}
		var queryAsMap map[string]interface{}
		err2 := json.Unmarshal([]byte(parsedQuery), &queryAsMap)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			return nil, err2
		}
		return &queryAsMap, nil
	}
	return nil, nil
}
