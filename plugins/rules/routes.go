package rules

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

func (r *Rules) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Create a query rule",
			Methods:     []string{http.MethodPost},
			Path:        "/_rule",
			HandlerFunc: middleware(r.postRule()),
			Description: "Creates a new query rule for a given index",
		},
		{
			Name:        "Update a query rule",
			Methods:     []string{http.MethodPut},
			Path:        "/_rule/{id}",
			HandlerFunc: middleware(r.putRule()),
			Description: "Updates query rule for a given {id}",
		},
		{
			Name:        "Delete a query rule",
			Methods:     []string{http.MethodDelete},
			Path:        "/_rule/{id}",
			HandlerFunc: middleware(r.deleteRule()),
			Description: "Deletes the rule with the given {id}",
		},
		{
			Name:        "Get a query rule",
			Methods:     []string{http.MethodGet},
			Path:        "/_rule/{id}",
			HandlerFunc: middleware(r.getRule()),
			Description: "Gets the rule with the given {id}",
		},
		{
			Name:        "Get query rules",
			Methods:     []string{http.MethodGet},
			Path:        "/_rules",
			HandlerFunc: middleware(r.getRules()),
			Description: "Gets the all rules}",
		},
		{
			Name:        "Validate the script action",
			Methods:     []string{http.MethodPost},
			Path:        "/_script/validate",
			HandlerFunc: middleware(r.validateScript()),
			Description: "Validates and apply a Javascript script to RS API",
		},
		{
			Name:        "Gets the script for a rule",
			Methods:     []string{http.MethodGet},
			Path:        "/_rule/{id}/script",
			HandlerFunc: middleware(r.getRuleScript()),
			Description: "To retrieve a script for a particular rule",
		},
	}
}
