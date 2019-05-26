package classify

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/index"
	"github.com/appbaseio-confidential/arc/util"
)

// Indices returns a middleware that identifies the indices present in the es route.
func Indices() middleware.Middleware {
	return indices
}

func indices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indices := util.IndicesFromRequest(req)

		ctx := index.NewContext(req.Context(), indices)
		req = req.WithContext(ctx)

		h(w, req)
	}
}
