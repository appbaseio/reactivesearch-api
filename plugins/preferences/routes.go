package preferences

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

func (rx *preferences) routes() []plugins.Route {
	routes := []plugins.Route{
		{
			Name:        "Get Preferences",
			Methods:     []string{http.MethodGet},
			Path:        "/{index}/_preferences",
			HandlerFunc: getPreferencesHandler(),
			Description: "Get the preferences for a particular index",
		},
		{
			Name:        "Save Preferences",
			Methods:     []string{http.MethodPut},
			Path:        "/{index}/_preferences",
			HandlerFunc: savePreferencesHandler(),
			Description: "Saves the preferences for a particular index",
		},
	}
	return routes
}
