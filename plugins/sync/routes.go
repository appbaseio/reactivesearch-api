package sync

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (s *Sync) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Save sync Preferences",
			Methods:     []string{http.MethodPut},
			Path:        "/_sync/preferences",
			HandlerFunc: middleware(s.savePreferencesHandler()),
			Description: "Update the preferences for sync plugin",
		},
		{
			Name:        "Get sync Preferences",
			Methods:     []string{http.MethodGet},
			Path:        "/_sync/preferences",
			HandlerFunc: middleware(s.getPreferencesHandler()),
			Description: "Retrieve the preferences for sync plugin",
		},
	}
}
