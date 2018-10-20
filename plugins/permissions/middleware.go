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

		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser == nil {
			log.Printf("%s: cannot fetch user object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		u := ctxUser.(*user.User)

		ctxOp := ctx.Value(op.CtxKey)
		if ctxOp == nil {
			log.Printf("%s: cannot fetch op from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		operation := ctxOp.(*op.Operation)

		if !op.Contains(u.Ops, *operation) {
			msg := fmt.Sprintf(`User with "user_id"="%s" does not have "%s" op`, u.UserId, operation.String())
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser == nil {
			log.Printf("%s: cannot fetch user from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		u := ctxUser.(*user.User)

		if !acl.Contains(u.ACLs, acl.Permission) {
			msg := fmt.Sprintf(`User with "user_id"="%s" does not have "%s" acl`, u.UserId, acl.Permission.String())
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func isAdmin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser == nil {
			log.Printf("%s: cannot fetch user from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		u := ctxUser.(*user.User)
		if !*u.IsAdmin {
			msg := fmt.Sprintf(`User with "user_id"="%s" is not an admin`, u.UserId)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}
