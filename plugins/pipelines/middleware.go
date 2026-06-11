package pipelines

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func (c *chain) WrapPipelineStageMw(h http.HandlerFunc, mw []middleware.Middleware) http.HandlerFunc {
	return c.Adapt(h, mw...)
}

// A list of plans for which this feature will be available
var validPlans = []util.Plan{
	util.ArcEnterprise,
	util.HostedArcEnterprise,
	util.ProductionFirst2019,
	util.ProductionSecond2019,
	util.ProductionThird2019,
	util.ProductionFourth2019,
	// 2021 plans
	util.ProductionFirst2021,
	util.ProductionSecond2021,
	util.ProductionThird2021,
	util.HostedArcEnterprise2021,
	util.Sandbox2023,
	util.Starter2023,
	util.ProductionFirst2023,
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classifyIndices,
		logs.Recorder(),
		classify.Op(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Operation(),
		validate.Category(),
		validate.Plan(validPlans, util.GetFeaturePipelines(), "Pipeline feature"),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		pipelineCategory := category.Pipelines

		ctx := category.NewContext(req.Context(), &pipelineCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func (route *ESPipelineRoutes) classifyRouteCategory() func(h http.HandlerFunc) http.HandlerFunc {
	return func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			// TODO: Change it to default category, category is required
			routeCategory := category.Pipelines
			if route.Classify != nil && route.Classify.Category != nil {
				routeCategory = *route.Classify.Category
			}
			ctx := category.NewContext(req.Context(), &routeCategory)
			req = req.WithContext(ctx)

			h(w, req)
		}
	}
}

func (route *ESPipelineRoutes) classifyRouteACL() func(h http.HandlerFunc) http.HandlerFunc {
	return func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			// TODO: Change it to default category, category is required
			if route.Classify != nil && route.Classify.ACL != nil {
				routeACL := *route.Classify.ACL
				ctx := acl.NewContext(req.Context(), &routeACL)
				req = req.WithContext(ctx)
			}
			h(w, req)
		}
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := index.NewContext(req.Context(), []string{util.MetaIndexName(defaultPipelinesEsIndex)})
		req = req.WithContext(ctx)
		h(w, req)
	}
}
