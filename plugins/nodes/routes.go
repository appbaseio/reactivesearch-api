package nodes

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (n *nodes) routes() []plugins.Route {
	routes := []plugins.Route{
		{
			Name:        "Arc Health Check",
			Methods:     []string{http.MethodGet, http.MethodHead, http.MethodPost},
			Path:        "/arc/_health",
			HandlerFunc: n.healthCheckNodes(),
			Description: "Return detail about the current node as well as active nodes",
		},
	}
	return routes
}
