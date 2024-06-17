package openai

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/middleware/ratelimiter"
	"github.com/appbaseio-confidential/reactivesearch/middleware/validate"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/model/op"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
}

// A list of plans for which this feature will be available
var validPlans = []util.Plan{
	util.Sandbox2023,
	util.Starter2023,
	util.ProductionFirst2023,
}

func GetValidOpenAIPlans() []util.Plan {
	return validPlans
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	// Append telemetry at the end
	mw := []middleware.Middleware{}
	mw = append(mw, telemetry.Recorder())
	return c.Adapt(h, append(list(), mw...)...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classify.Op(),
		classifyCategory,
		auth.BasicAuth(),
		classify.Indices(),
		ratelimiter.Limit(),
		validate.Sources(),
		validate.Referers(),
		validate.Category(),
		validate.Operation(),
		validate.Plan(validPlans, util.GetFeatureOpenAI(), "OpenAI feature"),
		validate.PermissionExpiry(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.AI

		route := mux.CurrentRoute(req)

		template, err := route.GetPathTemplate()
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "page not found", http.StatusNotFound)
			return
		}

		if (req.Method == http.MethodGet || req.Method == http.MethodPost) && (template == "/_ai/{AISessionId}" || template == "/_ai/{AISessionId}/sse") {
			requestCategory = category.ReactiveSearch

			// Also set the operation as `read` regardless of method.
			readOp := op.Read
			ctx := op.NewContext(req.Context(), &readOp)
			req = req.WithContext(ctx)
		}

		// Set analytics category for the analytics endpoints
		if (req.Method == http.MethodGet || req.Method == http.MethodPut) && (template == "/_ai/analytics" || template == "/_ai/analytics/filter" || template == "/_ai/{AISessionId}/detail" || template == "/_ai/{AISessionId}/analytics") {
			requestCategory = category.Analytics
		}

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}
