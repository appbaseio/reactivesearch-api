package users

import (
	"context"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/op"
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
			operation = op.Noop
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, op.CtxKey, operation)
		h(w, r.WithContext(ctx))
	}
}
