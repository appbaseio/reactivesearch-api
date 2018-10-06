package analytics

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/plugins/auth"
)

func (a *analytics) routes() []plugin.Route {
	basicAuth := auth.Instance().BasicAuth
	return []plugin.Route{
		{
			Name:        "Get overview",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/overview",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getOverview())))),
			Description: "Returns analytics overview on cluster",
		},
		{
			Name:        "Get advanced",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/advanced",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getAdvanced())))),
			Description: "Returns advanced analytics on cluster",
		},
		{
			Name:        "Get popular searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularsearches",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getPopularSearches())))),
			Description: "Returns popular searches on cluster",
		},
		{
			Name:        "Get no results searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/noresultssearches",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getNoResultsSearches())))),
			Description: "Returns no results searches on cluster",
		},
		{
			Name:        "Get popular filters",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularfilters",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getPopularFilters())))),
			Description: "Returns popular filters on cluster",
		},
		{
			Name:        "Get popular results",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularresults",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getPopularResults())))),
			Description: "Returns popular results on cluster",
		},
		{
			Name:        "Get geo ip",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/geodistribution",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getGeoRequestsDistribution())))),
			Description: "Returns search counts based on request/ip location on cluster",
		},
		{
			Name:        "Get latencies",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/latency",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getLatencies())))),
			Description: "Returns search latencies",
		},
		{
			Name:        "Get summary",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/summary",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(a.getSummary())))),
			Description: "Returns total searches, avg click and conversion rate on cluster",
		},
	}
}
