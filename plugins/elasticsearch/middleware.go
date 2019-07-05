package elasticsearch

import (
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/middleware/interceptor"
	"github.com/appbaseio/arc/middleware/ratelimiter"
	"github.com/appbaseio/arc/middleware/validate"
	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/plugins/auth"
	"github.com/appbaseio/arc/plugins/logs"
	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(mw [] middleware.Middleware, h http.HandlerFunc) http.HandlerFunc {
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
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		route := mux.CurrentRoute(req)

		template, err := route.GetPathTemplate()
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
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
			log.Printf("%s: %v\n", logTag, err)
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
			log.Printf("%s: %v\n", logTag, err)
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
