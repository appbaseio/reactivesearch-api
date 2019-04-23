package proxy

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/route"
)

func (px *Proxy) routes() []route.Route {
	//middleware := (&chain{}).Wrap
	routes := []route.Route{
		{
			Name:        "Create arc subscription",
			Methods:     []string{http.MethodPost},
			Path:        "/arc/subscription",
			HandlerFunc: px.postSubscription(),
			Description: "A proxy route to create ARC subscription.",
		},
		{
			Name:    "Delete arc subscription",
			Methods: []string{http.MethodDelete},
			Path:    "/arc/subscription",
			HandlerFunc: px.deleteSubscription(),
			Description: "A proxy route to delete ARC subscription.",
		},
		{
			Name:    "Get arc subscription",
			Methods: []string{http.MethodGet},
			Path:    "/arc/instance",
			HandlerFunc: px.getSubscription(),
			Description: "A proxy route to get ARC subscription details.",
		},
	}
	return routes
}
