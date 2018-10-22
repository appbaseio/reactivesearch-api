package permissions

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/appbaseio-confidential/arc/middleware/classifier"
	"github.com/appbaseio-confidential/arc/middleware/logger"
	"github.com/appbaseio-confidential/arc/middleware/path"
	"github.com/appbaseio-confidential/arc/plugins/auth"
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
	logRequests := logger.Instance().Log
	cleanPath := path.Clean

	return []middleware.Middleware{
		cleanPath,
		logRequests,
		opClassifier,
		aclClassifier,
		basicAuth,
		validateOp,
		validateACL,
	}
}

func aclClassifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		permissionACL := acl.Permission
		ctx := context.WithValue(r.Context(), acl.CtxKey, &permissionACL)
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
			msg := fmt.Sprintf(`User with "user_id"="%s" does not have "%s" op`, reqUser.UserID, *reqOp)
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
			util.WriteBackError(w, "An error occurred while validating request acl", http.StatusInternalServerError)
			return
		}

		if !reqUser.HasACL(acl.Permission) {
			msg := fmt.Sprintf(`User with "user_id"="%s" does not have "%s" acl`, reqUser.UserID, acl.Permission)
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
			msg := fmt.Sprintf(`User with "user_id"="%s" is not an admin`, reqUser.UserID)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}
