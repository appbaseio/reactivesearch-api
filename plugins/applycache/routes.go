package applycache

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

func (c *Cache) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Get Cache Analytics",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/cache",
			HandlerFunc: middleware(c.getCacheAnalyticsHandler()),
			Description: "Get analytics for cache like hit, miss and performance savings",
		},
	}
}
