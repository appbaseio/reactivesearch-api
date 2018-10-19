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

// aclClassifier classifies an incoming request based on the request method
// and the endpoint it is trying to access. The middleware makes two
// classifications: first, the operation (op.Operation) incoming request is
// trying to do, and second, the acl (acl.ACL) it is trying to access, which
// is acl.Permission in this case. The identified classifications are passed along
// in the request context to the next middleware. Classifier is supposedly
// the first middleware that a request is expected to encounter.
func aclClassifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		permissionACL := acl.Permission
		ctx := context.WithValue(r.Context(), acl.CtxKey, &permissionACL)
		r = r.WithContext(ctx)
		h(w, r)
	}
}

// validateOp verifies whether the permission.Permission has the required
// op.Operation in order to access a particular endpoint. The middleware
// expects the request context to have both *user.User who is making the
// request and *op.Operation required in order to access the endpoint. The
// absence of either values in request context will cause the middleware to
// return http.InternalServerError.
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
			msg := fmt.Sprintf("user with user_id=%s does not have '%s' op access",
				u.UserId, operation.String())
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

// validateACL verifies whether the user.User has the required acl.ACL in
// order to access a particular endpoint. The middleware expects the request
// context to have *user.User who is making the request. The absence of
// *user.User value in the request context will cause the middleware to return
// http.InternalServerError.
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
			msg := fmt.Sprintf(`user with "user_id"="%s" does not have '%s' acl`,
				u.UserId, acl.Permission.String())
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

// isAdmin checks whether the user.User is an admin. The middleware
// expects the request context to have a *user.User. The absence of *user.User
// in the request context will cause the middleware to return
// http.InternalServerError.
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
