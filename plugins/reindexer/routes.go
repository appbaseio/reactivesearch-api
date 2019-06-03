package reindexer

import (
	"net/http"

	"github.com/appbaseio/arc/plugins"
)

func (rx *reindexer) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	routes := []plugins.Route{
		{
			Name:        "Reindex",
			Methods:     []string{http.MethodPost},
			Path:        "/_reindex/{index}",
			HandlerFunc: middleware(rx.reindex()),
			Description: "Reindexes a single index with the given mappings, settings and types.",
		},
	}
	return routes
}
