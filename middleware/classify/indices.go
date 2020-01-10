package classify

import (
	"log"
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/model/index"
	"github.com/appbaseio/arc/util"
)

// Indices returns a middleware that identifies the indices present in the es route.
func Indices() middleware.Middleware {
	return indices
}

func indices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		log.Println("======================================MIDDLEWARE: CLASSIFY INDICES==================================")
		indices := util.IndicesFromRequest(req)

		ctx := index.NewContext(req.Context(), indices)
		req = req.WithContext(ctx)

		h(w, req)
	}
}
