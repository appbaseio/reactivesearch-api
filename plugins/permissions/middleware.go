package permissions

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
)

// classifier classifies an incoming request based on the request method
// and the endpoint it is trying to access. The middleware makes two
// classifications: first, the operation (op.Operation) incoming request is
// trying to do, and second, the acl (acl.ACL) it is trying to access, which
// is acl.Permission in this case. The identified classifications are passed along
// in the request context to the next middleware. Classifier is supposedly
// the first middleware that a request is expected to encounter.
func classifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		permissionACL := acl.Permission

		var operation op.Operation
		switch r.Method {
		case http.MethodGet:
			operation = op.Read
		case http.MethodPost:
			operation = op.Write
		case http.MethodPut:
			operation = op.Write
		case http.MethodHead:
			operation = op.Read
		case http.MethodDelete:
			operation = op.Delete
		default:
			operation = op.Read
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, acl.CtxKey, &permissionACL)
		ctx = context.WithValue(ctx, op.CtxKey, &operation)
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
