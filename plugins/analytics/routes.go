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
			Path:        "/_analytics/{index}/overview",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getOverview()))))),
			Description: "Returns analytics overview on an index or set of indices",
		},
		{
			Name:        "Get overview",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/overview",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getOverview()))))),
			Description: "Returns analytics overview on a cluster",
		},
		{
			Name:        "Get advanced",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/advanced",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getAdvanced()))))),
			Description: "Returns advanced analytics on an index or set of indices",
		},
		{
			Name:        "Get advanced",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/advanced",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getAdvanced()))))),
			Description: "Returns advanced analytics on a cluster",
		},
		{
			Name:        "Get popular searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/popularsearches",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getPopularSearches()))))),
			Description: "Returns popular searches on an or set of indices",
		},
		{
			Name:        "Get popular searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularsearches",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getPopularSearches()))))),
			Description: "Returns popular searches on a cluster",
		},
		{
			Name:        "Get no results searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/noresultssearches",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getNoResultsSearches()))))),
			Description: "Returns no results searches on an index or set of indices",
		},
		{
			Name:        "Get no results searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/noresultssearches",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getNoResultsSearches()))))),
			Description: "Returns no results searches on a cluster",
		},
		{
			Name:        "Get popular filters",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/popularfilters",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getPopularFilters()))))),
			Description: "Returns popular filters on an index or set of indices",
		},
		{
			Name:        "Get popular filters",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularfilters",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getPopularFilters()))))),
			Description: "Returns popular filters on a cluster",
		},
		{
			Name:        "Get popular results",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/popularresults",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getPopularResults()))))),
			Description: "Returns popular results on an index or set of indices",
		},
		{
			Name:        "Get popular results",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularresults",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getPopularResults()))))),
			Description: "Returns popular results on a cluster",
		},
		{
			Name:        "Get geo ip",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/geodistribution",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getGeoRequestsDistribution()))))),
			Description: "Returns search counts based on request/ip location on an index or set of indices",
		},
		{
			Name:        "Get geo ip",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/geodistribution",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getGeoRequestsDistribution()))))),
			Description: "Returns search counts based on request/ip location on a cluster",
		},
		{
			Name:        "Get search latencies",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/latency",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getSearchLatencies()))))),
			Description: "Returns search latencies for requests made on an index or set of indices",
		},
		{
			Name:        "Get search latencies",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/latency",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getSearchLatencies()))))),
			Description: "Returns search latencies for requests made on a cluster",
		},
		{
			Name:        "Get summary",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/summary",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getSummary()))))),
			Description: "Returns total searches, avg click and conversion rates on an index or set of indices",
		},
		{
			Name:        "Get summary",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/summary",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(validateIndices(a.getSummary()))))),
			Description: "Returns total searches, avg click and conversion rates on a cluster",
		},
	}
}
