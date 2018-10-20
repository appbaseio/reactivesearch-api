package analytics

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (a *Analytics) routes() []plugin.Route {
	middleware := (&chain{}).Wrap
	return []plugin.Route{
		{
			Name:        "Get overview",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/overview",
			HandlerFunc: middleware(a.getOverview()),
			Description: "Returns analytics overview of an index or set of indices",
		},
		{
			Name:        "Get overview",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/overview",
			HandlerFunc: middleware(a.getOverview()),
			Description: "Returns analytics overview of a cluster",
		},
		{
			Name:        "Get advanced",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/advanced",
			HandlerFunc: middleware(a.getAdvanced()),
			Description: "Returns advanced analytics of an index or set of indices",
		},
		{
			Name:        "Get advanced",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/advanced",
			HandlerFunc: middleware(a.getAdvanced()),
			Description: "Returns advanced analytics of a cluster",
		},
		{
			Name:        "Get popular searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/popular-searches",
			HandlerFunc: middleware(a.getPopularSearches()),
			Description: "Returns popular searches on an or set of indices",
		},
		{
			Name:        "Get popular searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popular-searches",
			HandlerFunc: middleware(a.getPopularSearches()),
			Description: "Returns popular searches on a cluster",
		},
		{
			Name:        "Get no result searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/no-result-searches",
			HandlerFunc: middleware(a.getNoResultSearches()),
			Description: "Returns no result searches on an index or set of indices",
		},
		{
			Name:        "Get no result searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/no-result-searches",
			HandlerFunc: middleware(a.getNoResultSearches()),
			Description: "Returns no result searches on a cluster",
		},
		{
			Name:        "Get popular filters",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/popular-filters",
			HandlerFunc: middleware(a.getPopularFilters()),
			Description: "Returns popular filters on an index or set of indices",
		},
		{
			Name:        "Get popular filters",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popular-filters",
			HandlerFunc: middleware(a.getPopularFilters()),
			Description: "Returns popular filters on a cluster",
		},
		{
			Name:        "Get popular results",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/popular-results",
			HandlerFunc: middleware(a.getPopularResults()),
			Description: "Returns popular results on an index or set of indices",
		},
		{
			Name:        "Get popular results",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popular-results",
			HandlerFunc: middleware(a.getPopularResults()),
			Description: "Returns popular results on a cluster",
		},
		{
			Name:        "Get geo distribution",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/geo-distribution",
			HandlerFunc: middleware(a.getGeoRequestsDistribution()),
			Description: "Returns search counts based on request/ip location on an index or set of indices",
		},
		{
			Name:        "Get geo distribution",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/geo-distribution",
			HandlerFunc: middleware(a.getGeoRequestsDistribution()),
			Description: "Returns search counts based on request/ip location on a cluster",
		},
		{
			Name:        "Get search latencies",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/latency",
			HandlerFunc: middleware(a.getSearchLatencies()),
			Description: "Returns search latencies for requests made on an index or set of indices",
		},
		{
			Name:        "Get search latencies",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/latency",
			HandlerFunc: middleware(a.getSearchLatencies()),
			Description: "Returns search latencies for requests made on a cluster",
		},
		{
			Name:        "Get summary",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/{index}/summary",
			HandlerFunc: middleware(a.getSummary()),
			Description: "Returns total searches, avg click and conversion rates on an index or set of indices",
		},
		{
			Name:        "Get summary",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/summary",
			HandlerFunc: middleware(a.getSummary()),
			Description: "Returns total searches, avg click and conversion rates on a cluster",
		},
	}
}
