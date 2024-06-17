package synonyms

import (
	"net/http"
	"strings"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/middleware/ratelimiter"
	"github.com/appbaseio-confidential/reactivesearch/middleware/validate"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/model/index"
	"github.com/appbaseio-confidential/reactivesearch/plugins/auth"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/gorilla/mux"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

// A list of plans for which synonyms will be enabled
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
		classify.Op(),
		classify.Indices(),
		classifyDeleteRouteIndex,
		logs.Recorder(),
		auth.BasicAuth(),
		validate.Sources(),
		ratelimiter.Limit(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
		validate.Plan(validPlans, util.GetFeatureSearchRelevancy(), "Synonyms feature"),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.Synonyms

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

// classifyDeleteRouteIndex will classify the delete route's index
// as the index for that is not passed as a route variable but will have
// to be instead extracted from the synonym ID.
func classifyDeleteRouteIndex(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodDelete {
			h(w, req)
			return
		}

		// It is the DELETE synonym route and we will have to extract
		// the index from the ID and inject it into the context.
		vars := mux.Vars(req)
		synonymId := vars["id"]

		idSplittedByIndexKey := strings.Split(synonymId, "__index__")
		splittedArrLen := len(idSplittedByIndexKey)

		// The ID should be of the format <id>__index__<index>
		// so the array should be at-least of length 2.
		if splittedArrLen < 2 {
			h(w, req)
			return
		}

		indexName := idSplittedByIndexKey[splittedArrLen-1]

		// Inject the index name into the context.
		ctx := index.NewContext(req.Context(), []string{indexName})
		req = req.WithContext(ctx)

		h(w, req)
	}
}
