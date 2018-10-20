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

		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser == nil {
			log.Printf("%s: cannot fetch user object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqUser := ctxUser.(*user.User)

		ctxOp := ctx.Value(op.CtxKey)
		if ctxOp == nil {
			log.Printf("%s: cannot fetch op from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		operation := ctxOp.(*op.Operation)

		if !reqUser.Can(*operation) {
			msg := fmt.Sprintf(`User with "user_id"="%s" does not have "%s" op`, reqUser.UserId, operation.String())
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
			log.Printf("%s: cannot fetch user object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqUser := ctxUser.(*user.User)

		if !reqUser.HasACL(acl.User) {
			msg := fmt.Sprintf(`User with "user_id"="%s" does not have '%s' acl`, reqUser.UserId, acl.User.String())
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
		reqUser := ctxUser.(*user.User)

		if !*reqUser.IsAdmin {
			msg := fmt.Sprintf(`User with "user_id"="%s" is not an admin`, reqUser.UserId)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}
