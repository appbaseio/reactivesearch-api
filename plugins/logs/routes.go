package logs

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/route"
)

func (l *Logs) routes() []route.Route {
	return []route.Route{
		{
			Name:        "Get index logs",
			Methods:     []string{http.MethodGet},
			Path:        "/{index}/logs",
			HandlerFunc: l.getLogs(),
			Description: "Returns the logs for an index",
		},
		{
			Name:        "Get logs",
			Methods:     []string{http.MethodGet},
			Path:        "/logs",
			HandlerFunc: l.getLogs(),
			Description: "Returns the logs for the cluster",
		},
	}
}
