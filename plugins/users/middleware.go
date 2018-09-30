package users

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

func classifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		ctx = context.WithValue(ctx, acl.CtxKey, acl.User)
		ctx = context.WithValue(ctx, op.CtxKey, operation)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		u := ctx.Value(user.CtxKey).(*user.User)
		if u == nil {
			log.Printf("%s: cannot fetch user object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		o := ctx.Value(op.CtxKey)
		if o == nil {
			log.Printf("%s: cannot fetch op from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !op.Contains(u.Ops, o.(op.Operation)) {
			msg := fmt.Sprintf("user with user_id=%s does not have '%s' op access",
				u.UserId, o.(op.Operation).String())
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		log.Printf("%s: validateOp: validated\n", logTag)
		h(w, r)
	}
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		u := ctx.Value(user.CtxKey).(*user.User)
		if u == nil {
			// TODO: auth didn't fetch user?
			log.Printf("%s: cannot fetch user object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !acl.Contains(u.ACLs, acl.Permission) {
			msg := fmt.Sprintf("user with user_id=%s does not have 'permission' acl", u.UserId)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		log.Printf("%s: validate acl: validated", logTag)
		h(w, r)
	}
}
