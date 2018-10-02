package analytics

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (a *Analytics) routes() []plugin.Route {
	return []plugin.Route{
		{
			Name:        "Get latency",
			Methods:     []string{http.MethodGet},
			Path:        "/_analytics/latency",
			HandlerFunc: a.getLatency(),
			Description: "",
		},
	}
}
