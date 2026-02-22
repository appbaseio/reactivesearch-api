package querytranslate

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
)

var (
	routes []plugins.Route
)

func (c *chain) ValidateWrap(h http.HandlerFunc) http.HandlerFunc {
	// Save request to ctx
	mw := []middleware.Middleware{saveRequestToCtx}
	// Append query translate middleware at the end
	mw = append(mw, queryTranslate)
	return c.Adapt(h, mw...)
}

func (px *QueryTranslate) routes() []plugins.Route {
	middlewareFunction := (&chain{}).ValidateWrap
	routes = append(routes, plugins.Route{
		Name:        "To get the API schema for ReactiveSearch",
		Methods:     []string{http.MethodGet},
		Path:        "/_reactivesearch/schema",
		HandlerFunc: px.HandleApiSchema(),
		Description: "Get the API schema for ReactiveSearch endpoint.",
	})
	routes = append(routes, plugins.Route{
		Name:        "To validate reactivesearch query",
		Methods:     []string{http.MethodPost},
		Path:        "/_reactivesearch.v3/validate",
		HandlerFunc: middlewareFunction(px.validate()), // Validate route is an open route, don't apply middleware on it
		Description: "Validates the query props and returns the query DSL.",
	})
	// Routes without v3 suffix
	routes = append(routes, plugins.Route{
		Name:        "To validate reactivesearch query",
		Methods:     []string{http.MethodPost},
		Path:        "/_reactivesearch/validate",
		HandlerFunc: middlewareFunction(px.validate()), // Validate route is an open route, don't apply middleware on it
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
	// Routes without v3 suffix
	routes = append(routes, plugins.Route{
		Name:        "Proxy to elasticsearch _msearch",
		Methods:     []string{http.MethodPost},
		Path:        "/{index}/_reactivesearch",
		HandlerFunc: middlewareFunction(mw, px.search()),
		Description: "A proxy route to handle search request based on the query props.",
	})
	// Add validate route at index level
	routes = append(routes, plugins.Route{
		Name:        "Proxy to elasticsearch _msearch",
		Methods:     []string{http.MethodPost},
		Path:        "/{index}/_reactivesearch/validate",
		HandlerFunc: middlewareFunction(mw, px.validate()),
		Description: "A proxy route to handle search request based on the query props.",
	})
	return nil
}
