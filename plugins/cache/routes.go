package cache

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (c *Cache) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Save Preferences",
			Methods:     []string{http.MethodPost},
			Path:        "/_cache/preferences",
			HandlerFunc: middleware(c.savePreferencesHandler()),
			Description: "Saves the cache preferences in ES index and update arc state",
		},
		{
			Name:        "Clear Cached Memory and Trigger update for other nodes",
			Methods:     []string{http.MethodPost},
			Path:        "/_cache/evict",
			HandlerFunc: middleware(c.clearCacheHandler()),
			Description: "Clear the cache and call ACCAPI to update other nodes",
		},
		{
			Name:        "Get Preferences",
			Methods:     []string{http.MethodGet},
			Path:        "/_cache/preferences",
			HandlerFunc: middleware(c.getPreferencesHandler()),
			Description: "To retrieve the saved cache preferences",
		},
	}
}
