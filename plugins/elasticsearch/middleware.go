package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/ratelimiter"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/domain"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/sourcefilter"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gorilla/mux"
)

type MGetResponse struct {
	Docs []map[string]interface{} `json:"docs"`
}

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(mw []middleware.Middleware, h http.HandlerFunc) http.HandlerFunc {
	// Append telemetry middleware at the end
	return c.Adapt(h, append(append(list(), mw...), telemetry.Recorder())...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{

		validate.Elasticsearch(),
		classifyCategory,
		classifyACL,
		classifyOp,
		classify.Indices(),
		logs.Recorder(),
		auth.BasicAuth(),
		ratelimiter.Limit(),
		validate.Sources(),
		validate.Referers(),
		validate.Indices(),
		validate.Category(),
		validate.ACL(),
		validate.Operation(),
		validate.PermissionExpiry(),
		intercept,
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		route := mux.CurrentRoute(req)

		template, err := route.GetPathTemplate()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "page not found", http.StatusNotFound)
			return
		}
		key := fmt.Sprintf("%s:%s", req.Method, template)
		routeSpec := routeSpecs[key]
		routeCategory := routeSpec.category

		// classify streams explicitly
		stream := req.Header.Get("X-Request-Category")
		if stream == "streams" {
			routeCategory = category.Streams
		}

		ctx := category.NewContext(req.Context(), &routeCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		currentRoute := mux.CurrentRoute(req)

		template, err := currentRoute.GetPathTemplate()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "page not found", http.StatusNotFound)
			return
		}
		key := fmt.Sprintf("%s:%s", req.Method, template)
		routeSpec := routeSpecs[key]
		routeACL := routeSpec.acl

		ctx := acl.NewContext(req.Context(), &routeACL)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		route := mux.CurrentRoute(req)

		template, err := route.GetPathTemplate()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "page not found", http.StatusNotFound)
			return
		}
		key := fmt.Sprintf("%s:%s", req.Method, template)
		routeSpec := routeSpecs[key]
		routeOp := routeSpec.op

		ctx := op.NewContext(req.Context(), &routeOp)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func shouldApplyFilters(reqPermission *permission.Permission) bool {
	isIncludesPresent := len(reqPermission.Includes) > 0
	isExcludesPresent := len(reqPermission.Excludes) > 0
	isEmpty := !isIncludesPresent && !isExcludesPresent
	isDefaultInclude := isIncludesPresent && reqPermission.Includes[0] == "*"
	return !isEmpty && (!isDefaultInclude || isExcludesPresent)
}

func intercept(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
		}
		isMsearch := *reqACL == acl.Msearch
		isSearch := *reqACL == acl.Search
		if (isSearch || isMsearch) && !strings.Contains(req.URL.Path, "/scroll") {
			// Apply source filters
			// /_search/scroll is a special case that doesn't support source filtering
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Warnln(logTag, ":", err)
			} else {
				shouldApplyFilters := shouldApplyFilters(reqPermission)
				if shouldApplyFilters {
					sources := make(map[string]interface{})
					var Includes, Excludes []string
					Includes = reqPermission.Includes
					Excludes = reqPermission.Excludes
					if len(Includes) > 0 {
						sources["includes"] = Includes
					}
					if len(Excludes) > 0 {
						sources["excludes"] = Excludes
					}
					if isMsearch {
						// Handle the _msearch requests
						body, err := ioutil.ReadAll(req.Body)
						if err != nil {
							log.Errorln(logTag, ":", err)
							telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
							return
						}
						var reqBodyString = string(body)
						splitReq := strings.Split(reqBodyString, "\n")
						var modifiedBodyString string
						for index, element := range splitReq {
							if index%2 == 1 { // even lines
								var reqBody = make(map[string]interface{})
								err := json.Unmarshal([]byte(element), &reqBody)
								if err != nil {
									log.Errorln(logTag, ":", err)
									telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
									return
								}
								reqBody["_source"] = sources
								raw, err := json.Marshal(reqBody)
								if err != nil {
									log.Errorln(logTag, ":", err)
									telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
									return
								}
								modifiedBodyString += string(raw)
							} else {
								modifiedBodyString += element
							}
							modifiedBodyString += "\n"
						}
						modifiedBody := []byte(modifiedBodyString)
						req.Body = ioutil.NopCloser(bytes.NewReader(modifiedBody))
					} else {
						reqBody := make(map[string]interface{})
						err := json.NewDecoder(req.Body).Decode(&reqBody)
						if err != nil && err != io.EOF {
							log.Errorln(logTag, ":", err)
							telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
							return
						}
						reqBody["_source"] = sources
						modifiedBody, _ := json.Marshal(reqBody)
						req.Body = ioutil.NopCloser(bytes.NewReader(modifiedBody))
					}
				}
			}
		}

		resp := httptest.NewRecorder()
		indices, err := index.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
		}
		h(resp, req)

		// Copy the response to writer
		for k, v := range resp.Header() {
			w.Header()[k] = v
		}
		result := resp.Result()
		body, err2 := ioutil.ReadAll(result.Body)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error reading response body", http.StatusInternalServerError)
			return
		}

		reqPermission, err := permission.FromContext(ctx)
		if err == nil && result.StatusCode == http.StatusOK {
			// Apply Appbase source filtering to the following type of requests
			// GET /:index/_doc/:id
			// GET /:index/_doc/:id/_source
			// GET /:index/_source/:id
			// GET /_mget
			// GET /:index/_mget
			shouldApplyFilters := shouldApplyFilters(reqPermission)
			if shouldApplyFilters {
				isDoc := strings.Contains(req.RequestURI, "_doc")
				isSource := strings.Contains(req.RequestURI, "_source")
				if *reqACL == acl.Get || *reqACL == acl.Source {
					if isDoc || isSource {
						var responseAsMap map[string]interface{}
						err := json.Unmarshal(body, &responseAsMap)
						if err != nil {
							log.Errorln(logTag, ":", err2)
							telemetry.WriteBackErrorWithTelemetry(req, w, "error un-marshalling _doc response", http.StatusInternalServerError)
							return
						}
						if isSource {
							filteredSource := sourcefilter.ApplySourceFiltering(responseAsMap, reqPermission.Includes, reqPermission.Excludes)
							if filteredSource != nil {
								responseAsMap = filteredSource.(map[string]interface{})
							} else {
								responseAsMap = make(map[string]interface{})
							}
							// Convert filtered response to byte
							filteredResponseInBytes, err := json.Marshal(responseAsMap)
							if err != nil {
								log.Errorln(logTag, ":", err2)
								telemetry.WriteBackErrorWithTelemetry(req, w, "error marshalling response", http.StatusInternalServerError)
								return
							}
							// Assign the filtered source to body
							body = filteredResponseInBytes
						} else {
							sourceAsMap, ok := responseAsMap["_source"].(map[string]interface{})
							if !ok {
								errMsg := "unable to type cast source to map[string]interface{}"
								log.Errorln(logTag, ":", errMsg)
								telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
								return
							}
							filteredSource := sourcefilter.ApplySourceFiltering(sourceAsMap, reqPermission.Includes, reqPermission.Excludes)
							if filteredSource == nil {
								filteredSource = make(map[string]interface{})
							}
							// Convert filtered response to byte
							filteredSourceInBytes, err := json.Marshal(filteredSource)
							if err != nil {
								log.Errorln(logTag, ":", err2)
								telemetry.WriteBackErrorWithTelemetry(req, w, "error marshalling response", http.StatusInternalServerError)
								return
							}
							filteredResponseInBytes, err := jsonparser.Set(body, filteredSourceInBytes, "_source")
							if err != nil {
								log.Errorln(logTag, ":", err2)
								telemetry.WriteBackErrorWithTelemetry(req, w, "error setting _source key in response", http.StatusInternalServerError)
								return
							}
							// Assign the filtered source to body
							body = filteredResponseInBytes
						}
					}
				}
				if *reqACL == acl.Mget {
					var mGetResponse MGetResponse
					err := json.Unmarshal(body, &mGetResponse)
					if err != nil {
						log.Errorln(logTag, ":", err2)
						telemetry.WriteBackErrorWithTelemetry(req, w, "error un-marshalling response", http.StatusInternalServerError)
						return
					}
					for _, doc := range mGetResponse.Docs {
						sourceAsMap, ok := doc["_source"].(map[string]interface{})
						if !ok {
							errMsg := "unable to type cast source to map[string]interface{}"
							log.Errorln(logTag, ":", errMsg)
							telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
							return
						}
						filteredSource := sourcefilter.ApplySourceFiltering(sourceAsMap, reqPermission.Includes, reqPermission.Excludes)
						if filteredSource != nil {
							doc["_source"] = filteredSource
						} else {
							doc["_source"] = make(map[string]interface{})
						}
					}
					// Convert filtered response to byte
					filteredResponseInBytes, err := json.Marshal(mGetResponse)
					if err != nil {
						log.Errorln(logTag, ":", err2)
						telemetry.WriteBackErrorWithTelemetry(req, w, "error marshalling response", http.StatusInternalServerError)
						return
					}
					// Assign the filtered source to body
					body = filteredResponseInBytes
				}
			}
		}
		for _, index := range indices {
			alias := classify.GetIndexAlias(index)
			if alias != "" {
				body = bytes.Replace(body, []byte(`"`+index+`"`), []byte(`"`+alias+`"`), -1)
				continue
			}
			// if alias is present in url get index name from cache
			indexName := classify.GetAliasIndex(index)
			if indexName != "" {
				body = bytes.Replace(body, []byte(`"`+indexName+`"`), []byte(`"`+index+`"`), -1)
			}
		}
		util.WriteBackRaw(w, body, result.StatusCode)
	}
}

// UpdateIndexName will update the index name by appending
// the tenantId to the name of the index for all whitelisted
// system routes that have {index} in the route
//
// It will also take care of updating the tenant cache with the
// deleted/created index.
func (wh *WhitelistedRoute) UpdateIndexName(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// This middleware will only be executed if SLS is enabled
		// and multi-tenant is enabled
		if util.IsSLSDisabled() || !util.MultiTenant {
			h(w, req)
			return
		}

		// Disable this middleware if the backend is not system
		// Fetch the domain from context
		domainUsed, domainFetchErr := domain.FromContext(req.Context())
		if domainFetchErr != nil {
			errMsg := "Error while validating the domain!"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusUnauthorized)
		}

		// Get the backend using the domain
		if *(util.GetBackendByDomain(domainUsed.Raw)) != util.System {
			// No need to blacklist
			h(w, req)
			return
		}

		// Execution will reach this point only if the backend is `system`
		// and multi-tenant SLS is used.
		//
		// We already have a separate middleware that whitelists certain
		// routes based on the whitelisted route list.
		//
		// This means execution will reach this point only if the backend
		// is `system` and the route is whitelisted so at this point
		// we can automatically replace the index name with the
		// tenant ID added index name

		// Access the vars to fetch the name of the index
		reqVars := mux.Vars(req)
		indexPassed := reqVars["index"]

		// TODO: Fetch the tenant ID using the domain
		tenantId := ""

		// Update the path
		indexWithTenant := util.AppendTenantID(indexPassed, tenantId)
		req.URL.Path = strings.Replace(req.URL.Path, indexPassed, indexWithTenant, -1)

		// If route is index and method is POST/DELETE, we need to update the tenant index cache
		if wh.Path == "/{index}" {
			if req.Method == http.MethodDelete {
				// Remove the entry from the cache
				DeleteIndexFromCache(tenantId, indexPassed)
			} else if req.Method == http.MethodPut {
				// Add the new entry in the cache
				SetIndexToCache(tenantId, indexPassed)
			}
		}

		// Serve using response recorder to capture the response
		//
		// We are doing this because we want to modify the outgoing response with
		// the name of the index that was passed instead of the one that
		// has the tenantId appended to it.
		respRecorder := httptest.NewRecorder()
		h(respRecorder, req)
		// Copy the response to writer
		for k, v := range respRecorder.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(respRecorder.Code)

		// Before writing the response, replace the index name to the one
		// that the user passed
		responseFromES := respRecorder.Body.Bytes()
		modifiedResponse := strings.Replace(string(responseFromES), indexWithTenant, indexPassed, -1)

		w.Write([]byte(modifiedResponse))
	}
}
