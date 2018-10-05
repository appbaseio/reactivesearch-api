package analytics

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (a *Analytics) routes() []plugin.Route {
	return []plugin.Route{
		{
			Name:        "Get popular searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularsearches",
			HandlerFunc: a.getPopularSearches(),
			Description: "Returns popular searches on cluster",
		},
		{
			Name:        "Get no results searches",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/noresultssearches",
			HandlerFunc: a.getNoResultsSearches(),
			Description: "Returns no results searches on cluster",
		},
		{
			Name:        "Get summary",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/summary",
			HandlerFunc: a.getSummary(),
			Description: "Returns total searches, avg click and conversion rate on cluster",
		},
		{
			Name:        "Get popular filters",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularfilters",
			HandlerFunc: a.getPopularFilters(),
			Description: "Returns popular filters on cluster",
		},
		{
			Name:        "Get popular results",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/popularresults",
			HandlerFunc: a.getPopularResults(),
			Description: "Returns popular results on cluster",
		},
		{
			Name:        "Get overview",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/overview",
			HandlerFunc: a.getOverview(),
			Description: "Returns analytics overview on cluster",
		},
		{
			Name:        "Get advanced",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/advanced",
			HandlerFunc: a.getAdvanced(),
			Description: "Returns advanced analytics on cluster",
		},
		{
			Name:        "Get geo ip",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/geoip",
			HandlerFunc: a.getGeoRequestsDistribution(),
			Description: "Returns search counts based on request/ip location on cluster",
		},
		{
			Name:        "Get latencies",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/latency",
			HandlerFunc: a.getLatencies(),
			Description: "Returns search latencies",
		},
	}
}
