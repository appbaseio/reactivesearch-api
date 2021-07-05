package querytranslate

import (
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/plugins"
)

var (
	routes []plugins.Route
)

func (px *QueryTranslate) routes() []plugins.Route {
	routes = append(routes, plugins.Route{
		Name:        "To validate reactivesearch query",
		Methods:     []string{http.MethodPost},
		Path:        "/_reactivesearch.v3/validate",
		HandlerFunc: px.validate(), // Validate route is an open route, don't apply middleware on it
		Description: "Validates the query props and returns the query DSL.",
	})
	return routes
}

func (px *QueryTranslate) preprocess(mw []middleware.Middleware) error {
	middlewareFunction := (&chain{}).Wrap
	routes = append(routes, plugins.Route{
		Name:        "Proxy to elasticsearch _msearch",
		Methods:     []string{http.MethodPost},
		Path:        "/{index}/_reactivesearch.v3",
		HandlerFunc: middlewareFunction(mw, px.search()),
		Description: "A proxy route to handle search request based on the query props.",
	})
	// Add validate route at index level
	routes = append(routes, plugins.Route{
		Name:        "Proxy to elasticsearch _msearch",
		Methods:     []string{http.MethodPost},
		Path:        "/{index}/_reactivesearch.v3/validate",
		HandlerFunc: middlewareFunction(mw, px.validate()),
		Description: "A proxy route to handle search request based on the query props.",
	})
	return nil
}
