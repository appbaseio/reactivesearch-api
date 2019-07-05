package classify

import (
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/model/op"
)

// Op returns a middleware that classifies request operation.
func Op() middleware.Middleware {
	return classifyOp
}

func classifyOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var operation op.Operation
		switch req.Method {
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

		ctx := op.NewContext(req.Context(), &operation)
		req = req.WithContext(ctx)

		h(w, req)
	}
}
