package searchgrader

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/middleware/validate"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/model/index"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
)

type chain struct {
	middleware.Fifo
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
	util.ProductionFirst2023,
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
		validate.Sources(),
		validate.Operation(),
		validate.Category(),
		validate.Plan(validPlans, util.GetFeatureSearchGrader(), "Search Grader feature"),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		searchGraderCategory := category.SearchGrader
		ctx := category.NewContext(req.Context(), &searchGraderCategory)
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := index.NewContext(req.Context(), []string{defaultSearchgraderEsIndex})
		req = req.WithContext(ctx)
		h(w, req)
	}
}
