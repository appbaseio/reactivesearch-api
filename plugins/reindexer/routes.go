package reindexer

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/route"
)

func (rx *reindexer) routes() []route.Route {
	routes := []route.Route{
		{
			Name:        "Reindex",
			Methods:     []string{http.MethodPost},
			Path:        "/_reindex/{index}",
			HandlerFunc: rx.reindex(),
			Description: "Reindexes a single index with the given mappings, settings and types.",
		},
	}
	return routes
}
