package permissions

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/middleware/classifier"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/op"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/plugins/auth"
	"github.com/appbaseio-confidential/arc/util"
)

type chain struct {
	order.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	basicAuth := auth.Instance().BasicAuth
	opClassifier := classifier.Instance().OpClassifier

	return []middleware.Middleware{
		opClassifier,
		aclClassifier,
		basicAuth,
		validateOp,
		validateACL,
	}
}

func aclClassifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		permissionACL := category.Permission
		ctx := context.WithValue(r.Context(), category.CtxKey, &permissionACL)
		r = r.WithContext(ctx)
		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request op"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !reqUser.CanDo(*reqOp) {
			msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" op`, reqUser.Username, *reqOp)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "An error occurred while validating request category", http.StatusInternalServerError)
			return
		}

		if !reqUser.HasCategory(category.Permission) {
			msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" category`, reqUser.Username, category.Permission)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func isAdmin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "An error occurred while validating user admin", http.StatusInternalServerError)
			return
		}

		if !(*reqUser.IsAdmin) {
			msg := fmt.Sprintf(`User with "username"="%s" is not an admin`, reqUser.Username)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}
