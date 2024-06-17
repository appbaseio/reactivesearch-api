package uibuilder

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/middleware/validate"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
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
	util.ProductionSecond2019,
	util.ProductionThird2019,
	util.ProductionFourth2019,
	// 2021 plans
	util.ProductionFirst2021,
	util.ProductionSecond2021,
	util.ProductionThird2021,
	util.HostedArcEnterprise2021,
	util.HostedArcStandard2021,
	util.Starter2021,
	util.ProductionFirst2023,
}

func (c *chain) Wrap(h http.HandlerFunc, isPremiumOnly bool) http.HandlerFunc {
	if isPremiumOnly {
		mw := append(list(), validate.Plan(validPlans, util.GetFeatureUIBuilderPremium(), "UI Builder auth/domain feature"))
		return c.Adapt(h, mw...)
	}
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classify.Op(),
		classify.Indices(),
		logs.Recorder(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.UIBuilder

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}
