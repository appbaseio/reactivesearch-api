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

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/middleware/interceptor"
	"github.com/appbaseio/arc/middleware/ratelimiter"
	"github.com/appbaseio/arc/middleware/validate"
	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/body"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/index"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/plugins/auth"
	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(mw []middleware.Middleware, h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, append(append(list(), mw...), interceptor.Redirect())...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classifyACL,
		classifyOp,
		classify.Indices(),
		// logs.Recorder(),
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
			util.WriteBackError(w, "page not found", http.StatusNotFound)
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
		h(w, req.WithContext(ctx))
	}
}

func classifyACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		currentRoute := mux.CurrentRoute(req)

		template, err := currentRoute.GetPathTemplate()
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "page not found", http.StatusNotFound)
			return
		}
		key := fmt.Sprintf("%s:%s", req.Method, template)
		routeSpec := routeSpecs[key]
		routeACL := routeSpec.acl

		ctx := acl.NewContext(req.Context(), &routeACL)
		h(w, req.WithContext(ctx))
	}
}

func classifyOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		route := mux.CurrentRoute(req)

		template, err := route.GetPathTemplate()
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "page not found", http.StatusNotFound)
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

func intercept(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
		}
		esBody, _ := body.FromContext(ctx)
		isMsearch := *reqACL == acl.Msearch
		isSearch := *reqACL == acl.Search
		// transform POST request(search) to GET
		if isSearch || isMsearch {
			fmt.Println("inside msearch condition")
			// Apply source filters
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Errorln(logTag, ":", err)
			} else {
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
				_, isExcludesPresent := sources["excludes"]
				isEmpty := len(Includes) == 0 && len(Excludes) == 0
				isDefaultInclude := len(Includes) > 0 && Includes[0] == "*"
				shouldApplyFilters := !isEmpty && (!isDefaultInclude || isExcludesPresent)
				fmt.Println("should apply filters: ", shouldApplyFilters)
				if shouldApplyFilters {
					if isMsearch {
						var reqBodyString = string(esBody)
						splitReq := strings.Split(reqBodyString, "\n")
						var modifiedBodyString string
						for index, element := range splitReq {
							if index%2 == 1 { // even lines
								var reqBody = make(map[string]interface{})
								err := json.Unmarshal([]byte(element), &reqBody)
								if err != nil {
									log.Errorln(logTag, ":", err)
									util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
									return
								}
								reqBody["_source"] = sources
								raw, err := json.Marshal(reqBody)
								if err != nil {
									log.Errorln(logTag, ":", err)
									util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
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
						d := json.NewDecoder(ioutil.NopCloser(bytes.NewReader(esBody)))
						reqBody := make(map[string]interface{})
						d.Decode(&reqBody)
						reqBody["_source"] = sources
						modifiedBody, _ := json.Marshal(reqBody)
						req.Body = ioutil.NopCloser(bytes.NewReader(modifiedBody))
					}
				}
			}
		}

		resp := httptest.NewRecorder()
		h(resp, req)

		// Copy the response to writer
		for k, v := range resp.Header() {
			w.Header()[k] = v
		}

		result := resp.Result()
		var body bytes.Buffer
		io.Copy(&body, result.Body)
		var bytesBody = body.Bytes()

		indices, err := index.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackRaw(w, bytesBody, http.StatusOK)
			//util.WriteBackError(w, "error getting indices from context", http.StatusInternalServerError)
			return
		}

		for _, index := range indices {
			alias := classify.GetIndexAlias(index)
			if alias != "" {
				bytesBody = bytes.Replace(bytesBody, []byte(`"`+index+`"`), []byte(`"`+alias+`"`), -1)
				continue
			}
			// if alias is present in url get index name from cache
			indexName := classify.GetAliasIndex(index)
			if indexName != "" {
				bytesBody = bytes.Replace(bytesBody, []byte(`"`+indexName+`"`), []byte(`"`+index+`"`), -1)
			}
		}
		util.WriteBackRaw(w, bytesBody, http.StatusOK)
	}
}
