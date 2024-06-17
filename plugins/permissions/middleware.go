package permissions

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/middleware/validate"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
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
		logs.Recorder(),
		classify.Op(),
		classify.Indices(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Operation(),
		validate.Category(),
		telemetry.Recorder(),
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
