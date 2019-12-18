package reindexer

import (
	"net/http"

	"github.com/appbaseio/arc/plugins"
)

func (rx *reindexer) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	routes := []plugins.Route{
		{
			Name:        "Reindex source to destination",
			Methods:     []string{http.MethodPost},
			Path:        "/_reindex/{source_index}/{destination_index}",
			HandlerFunc: middleware(rx.reindexSrcToDest()),
			Description: "Reindexes a index from given source to destination with the given mappings, settings and types.",
		},
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
