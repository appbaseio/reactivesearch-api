package elasticsearch

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/middleware/classify"
	"github.com/appbaseio-confidential/arc/middleware/interceptor"
	"github.com/appbaseio-confidential/arc/middleware/ratelimiter"
	"github.com/appbaseio-confidential/arc/middleware/validate"
	"github.com/appbaseio-confidential/arc/model/acl"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/op"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/plugins/analytics"
	"github.com/appbaseio-confidential/arc/plugins/auth"
	"github.com/appbaseio-confidential/arc/plugins/logs"
	"github.com/appbaseio-confidential/arc/plugins/rules"
	"github.com/appbaseio-confidential/arc/util"
	"github.com/gorilla/mux"
)

type chain struct {
	order.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
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
		validate.Operation(),
		validate.Category(),
		validate.ACL(),
		validate.PermissionExpiry(),
		analytics.Recorder(),
		rules.Apply(),
		interceptor.Redirect(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)

		template, err := route.GetPathTemplate()
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "page not found", http.StatusNotFound)
			return
		}
		key := fmt.Sprintf("%s:%s", r.Method, template)
		routeSpec := routeSpecs[key]
		routeCategory := routeSpec.category

		// classify streams explicitly
		params := r.URL.Query()
		stream := params.Get("stream")
		if stream == "true" {
			routeCategory = category.Streams
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, category.CtxKey, &routeCategory)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func classifyACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		currentRoute := mux.CurrentRoute(r)

		template, err := currentRoute.GetPathTemplate()
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "page not found", http.StatusNotFound)
			return
		}
		key := fmt.Sprintf("%s:%s", r.Method, template)
		routeSpec := routeSpecs[key]
		routeACL := routeSpec.acl

		ctx := r.Context()
		ctx = context.WithValue(ctx, acl.CtxKey, &routeACL)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func classifyOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)

		template, err := route.GetPathTemplate()
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "Page not found", http.StatusNotFound)
			return
		}
		key := fmt.Sprintf("%s:%s", r.Method, template)
		routeSpec := routeSpecs[key]
		routeOp := routeSpec.op

		ctx := r.Context()
		ctx = context.WithValue(ctx, op.CtxKey, &routeOp)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "an error occurred while validating request acl"
		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(r.Context())
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}
		if reqCredential == credential.User {
			reqUser, err := user.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !reqUser.HasACL(*reqACL) {
				msg := fmt.Sprintf(`user with "username"="%s" does not have "%s" acl`,
					reqUser.Username, *reqACL)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		} else if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			if !reqPermission.HasACL(*reqACL) {
				msg := fmt.Sprintf(`permission with "username"="%s" does not have "%s" acl`,
					reqPermission.Username, *reqACL)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, r)
	}
}

func validateCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request category"
		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}
		if reqCredential == credential.User {
			reqUser, err := user.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !reqUser.HasCategory(*reqCategory) {
				msg := fmt.Sprintf(`User with "username"="%s" does not have access to category "%s"`,
					reqUser.Username, *reqCategory)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		} else if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !reqPermission.HasCategory(*reqCategory) {
				msg := fmt.Sprintf(`Permission with "username"="%s" does not have access to category "%s"`,
					reqPermission.Username, reqCategory)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request op"
		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}
		if reqCredential == credential.User {
			reqUser, err := user.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !reqUser.CanDo(*reqOp) {
				msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" op`,
					reqUser.Username, *reqOp)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		} else if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}

			if !reqPermission.CanDo(*reqOp) {
				msg := fmt.Sprintf(`Permission with "username"="%s" does not have "%s" operation`,
					reqPermission.Username, *reqOp)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, r)
	}
}
