package users

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
	classifyOp := classifier.Instance().OpClassifier
	logRequests := logger.Instance().Log
	cleanPath := path.Clean

	return []middleware.Middleware{
		cleanPath,
		logRequests,
		classifyOp,
		classifyACL,
		basicAuth,
		validateOp,
		validateACL,
	}
}

func classifyACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userACL := acl.User
		ctx := context.WithValue(r.Context(), acl.CtxKey, &userACL)
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
			util.WriteBackError(w, "An error occurred while validating request acl", http.StatusInternalServerError)
		}

		if !reqUser.HasACL(acl.User) {
			msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" acl`, reqUser.Username, acl.User)
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

		if !*reqUser.IsAdmin {
			msg := fmt.Sprintf(`User with "username"="%s" is not an admin`, reqUser.Username)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}
