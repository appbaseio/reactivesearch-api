package rules

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/route"
)

func (r *Rules) routes() []route.Route {
	return []route.Route{
		{
			Name:        "Create a query rule",
			Methods:     []string{http.MethodPost},
			Path:        "/{index}/_rule",
			HandlerFunc: r.postIndexRule(),
			Description: "Creates a new query rule for a given index",
		},
		{
			Name:        "Create query rules",
			Methods:     []string{http.MethodPost},
			Path:        "/{index}/_rules",
			HandlerFunc: r.postIndexRules(),
			Description: "Creates query rules for a given index",
		},
		{
			Name:        "Get an index rule",
			Methods:     []string{http.MethodGet},
			Path:        "/{index}/_rule/{id}",
			HandlerFunc: r.getIndexRuleWithID(),
			Description: "Fetches the rule with the given {id}",
		},
		{
			Name:        "Get index rules",
			Methods:     []string{http.MethodGet},
			Path:        "/{index}/_rules",
			HandlerFunc: r.getIndexRules(),
			Description: "Fetches all the rules associated with an index",
		},
		{
			Name:        "Delete an index rule",
			Methods:     []string{http.MethodDelete},
			Path:        "/{index}/_rule/{id}",
			HandlerFunc: r.deleteIndexRuleWithID(),
			Description: "Deletes the rule with the given {id}",
		},
		{
			Name:        "Delete index rules",
			Methods:     []string{http.MethodDelete},
			Path:        "/{index}/_rules",
			HandlerFunc: r.deleteIndexRules(),
			Description: "Deletes all the rules associated with an index",
		},
	}
}
