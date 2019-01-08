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
			HandlerFunc: r.postRule(),
			Description: "Creates a new query rule for a given index",
		},
		{
			Name:        "Get index rules",
			Methods:     []string{http.MethodGet},
			Path:        "/{index}/_rules",
			HandlerFunc: r.getIndexRules(),
			Description: "Fetches all the rules associated with an index",
		},
	}
}
