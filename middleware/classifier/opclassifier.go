package classifier

import (
	"context"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/op"
)

func OpClassifier(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var operation op.Operation
		switch r.Method {
		case http.MethodGet:
			operation = op.Read
		case http.MethodPost:
			operation = op.Write
		case http.MethodPut:
			operation = op.Write
		case http.MethodPatch:
			operation = op.Write
		case http.MethodDelete:
			operation = op.Delete
		case http.MethodTrace:
			operation = op.Write
		default:
			operation = op.Read
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, op.CtxKey, &operation)
		r = r.WithContext(ctx)

		h(w, r)
	}
}
