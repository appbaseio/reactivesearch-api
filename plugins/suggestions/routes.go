package suggestions

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

func (rx *suggestions) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	routes := []plugins.Route{
		{
			Name:        "Get Popular Suggestions Preferences (deprecated)",
			Methods:     []string{http.MethodGet},
			Path:        "/_suggestions/preferences",
			HandlerFunc: middleware(rx.getPopularSuggestionsPreferences()),
			Description: "Gets the popular suggestions preferences",
		},
		{
			Name:        "Save Popular Suggestions Preferences (deprecated)",
			Methods:     []string{http.MethodPut},
			Path:        "/_suggestions/preferences",
			HandlerFunc: middleware(rx.savePopularSuggestionsPreferences()),
			Description: "Saves the popular suggestions preferences",
		},
		{
			Name:        "Get Popular Suggestions Preferences",
			Methods:     []string{http.MethodGet},
			Path:        "/_popular_suggestions/preferences",
			HandlerFunc: middleware(rx.getPopularSuggestionsPreferences()),
			Description: "Gets the popular suggestions preferences",
		},
		{
			Name:        "Save Popular Suggestions Preferences",
			Methods:     []string{http.MethodPut},
			Path:        "/_popular_suggestions/preferences",
			HandlerFunc: middleware(rx.savePopularSuggestionsPreferences()),
			Description: "Saves the popular suggestions preferences",
		},
		{
			Name:        "Get Index Suggestions Preferences",
			Methods:     []string{http.MethodGet},
			Path:        "/_index_suggestions/preferences",
			HandlerFunc: middleware(rx.getIndexSuggestionsPreferences()),
			Description: "Gets the index suggestions preferences",
		},
		{
			Name:        "Save Index Suggestions Preferences",
			Methods:     []string{http.MethodPut},
			Path:        "/_index_suggestions/preferences",
			HandlerFunc: middleware(rx.saveIndexSuggestionsPreferences()),
			Description: "Saves the index suggestions preferences",
		},
		{
			Name:        "Get Recent Suggestions Preferences",
			Methods:     []string{http.MethodGet},
			Path:        "/_recent_suggestions/preferences",
			HandlerFunc: middleware(rx.getRecentSuggestionsPreferences()),
			Description: "Gets the recent suggestions preferences",
		},
		{
			Name:        "Save Recent Suggestions Preferences",
			Methods:     []string{http.MethodPut},
			Path:        "/_recent_suggestions/preferences",
			HandlerFunc: middleware(rx.saveRecentSuggestionsPreferences()),
			Description: "Saves the recent suggestions preferences",
		},
	}
	return routes
}
