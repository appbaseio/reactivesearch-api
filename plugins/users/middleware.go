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

func opClassifier(h http.HandlerFunc) http.HandlerFunc {
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
			// TODO: handle?
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, op.CtxKey, operation)
		h(w, r.WithContext(ctx))
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

		if !op.Contains(u.Op, o.(op.Operation)) {
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

		if !acl.Contains(u.ACL, acl.Permission) {
			msg := fmt.Sprintf("user with user_id=%s does not have 'permission' acl", u.UserId)
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		log.Printf("%s: validate acl: validated", logTag)
		h(w, r)
	}
}
