package searchrelevancy

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (a *SearchRelevancy) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Get searchrelevancy",
			Methods:     []string{http.MethodGet},
			Path:        "/_searchrelevancy/{index}",
			HandlerFunc: middleware(a.getSearchRelevancySettings()),
			Description: "Returns searchrelevancy of an index",
		},
		{
			Name:        "Put searchrelevancy",
			Methods:     []string{http.MethodPut},
			Path:        "/_searchrelevancy/{index}",
			HandlerFunc: middleware(a.putSearchRelevancySettings()),
			Description: "Saves searchrelevancy of an index",
		},
		{
			Name:        "Delete searchrelevancy",
			Methods:     []string{http.MethodDelete},
			Path:        "/_searchrelevancy/{index}",
			HandlerFunc: middleware(a.deleteSearchRelevancySettings()),
			Description: "Deletes searchrelevancy of an index",
		},
	}
}
