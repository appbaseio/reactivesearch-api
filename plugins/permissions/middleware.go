package permissions

import (
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/middleware/validate"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/index"
	"github.com/appbaseio/arc/plugins/logs"
	"github.com/appbaseio/arc/plugins/auth"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classifyIndices,
		logs.Recorder(),
		classify.Op(),
		auth.BasicAuth(),
		validate.Operation(),
		validate.Category(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		permissionCategory := category.Permission

		ctx := category.NewContext(req.Context(), &permissionCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := index.NewContext(req.Context(), []string{defaultPermissionsEsIndex})
		req = req.WithContext(ctx)
		h(w, req)
	}
}
