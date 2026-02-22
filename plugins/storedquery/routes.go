package storedquery

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (r *StoredQuery) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Create/Update a stored query",
			Methods:     []string{http.MethodPut},
			Path:        "/_storedquery/{id}",
			HandlerFunc: middleware(r.putStoredQuery()),
			Description: "Creates/Updates stored query for a given {id}",
		},
		{
			Name:        "Delete a stored query",
			Methods:     []string{http.MethodDelete},
			Path:        "/_storedquery/{id}",
			HandlerFunc: middleware(r.deleteStoredQuery()),
			Description: "Deletes the stored query with the given {id}",
		},
		{
			Name:        "Validate a stored query",
			Methods:     []string{http.MethodPost},
			Path:        "/_storedquery/validate",
			HandlerFunc: middleware(r.validateStoredQuery()),
			Description: "Validate a stored query and returns the ES equivalent query",
		},
		{
			Name:        "Validate a stored query with a given id",
			Methods:     []string{http.MethodPost},
			Path:        "/_storedquery/{id}/validate",
			HandlerFunc: middleware(r.validateStoredQueryID()),
			Description: "Validate a stored query and returns the ES equivalent query",
		},
		{
			Name:        "Execute a stored query",
			Methods:     []string{http.MethodPost},
			Path:        "/_storedquery/execute",
			HandlerFunc: middleware(r.executeStoredQuery()),
			Description: "Execute a stored query and returns the ES output",
		},
		{
			Name:        "Execute a stored query with a given id",
			Methods:     []string{http.MethodPost},
			Path:        "/_storedquery/{id}/execute",
			HandlerFunc: middleware(r.executeStoredQueryID()),
			Description: "Execute a stored query and returns the ES output",
		},
		{
			Name:        "Get stored query",
			Methods:     []string{http.MethodGet},
			Path:        "/_storedquery/{id}",
			HandlerFunc: middleware(r.getStoredQuery()),
			Description: "Get the stored query for given id",
		},
		{
			Name:        "Get stored queries",
			Methods:     []string{http.MethodGet},
			Path:        "/_storedqueries",
			HandlerFunc: middleware(r.getStoredQueries()),
			Description: "Gets the all stored queries",
		},
	}
}
