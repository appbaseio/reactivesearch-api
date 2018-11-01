package reindexer

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (rx *reindexer) routes() []plugin.Route {
	routes := []plugin.Route{
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
