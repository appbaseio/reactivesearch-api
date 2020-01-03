package elasticsearch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/middleware/function"
	"github.com/appbaseio/arc/middleware/interceptor"
	"github.com/appbaseio/arc/middleware/ratelimiter"
	"github.com/appbaseio/arc/middleware/validate"
	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/plugins/auth"
	"github.com/appbaseio/arc/plugins/logs"
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
		function.Before(),
		// Call logs after that, update the existing log record
		transformRequest,
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

		ctx := req.Context()
		ctx = category.NewContext(req.Context(), &routeCategory)
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
			util.WriteBackError(w, "page not found", http.StatusNotFound)
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

func transformRequest(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		reqACL, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
		}
		// transform POST request(search) to GET
		if *reqACL == category.Search {
			isMsearch := strings.HasSuffix(req.URL.String(), "/_msearch")
			// Apply source filters
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Errorln(logTag, ":", err)
				h(w, req)
				return
			}
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
			isDefaultInclude := len(Includes) > 0 && Includes[0] == "*"
			shouldApplyFilters := !isDefaultInclude || isExcludesPresent
			if shouldApplyFilters {
				if isMsearch {
					// Handle the _msearch requests
					body, err := ioutil.ReadAll(req.Body)
					if err != nil {
						log.Errorln(logTag, ":", err)
						util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
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
								util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
								return
							}
							reqBody["_source"] = sources
							raw, _ := json.Marshal(reqBody)
							modifiedBodyString += string(raw)
						} else {
							modifiedBodyString += element
						}
						modifiedBodyString += "\n"
					}
					modifiedBody := []byte(modifiedBodyString)
					req.Body = ioutil.NopCloser(bytes.NewReader(modifiedBody))
				} else {
					body, err := ioutil.ReadAll(req.Body)
					if err != nil {
						log.Errorln(logTag, ":", err)
						util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
						return
					}
					d := json.NewDecoder(ioutil.NopCloser(bytes.NewReader(body)))
					reqBody := make(map[string]interface{})
					d.Decode(&reqBody)
					reqBody["_source"] = sources
					modifiedBody, _ := json.Marshal(reqBody)
					req.Body = ioutil.NopCloser(bytes.NewReader(modifiedBody))
				}
			}
		}
		h(w, req)
	}
}
