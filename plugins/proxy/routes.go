package proxy

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

func (px *Proxy) routes() []plugins.Route {
	//middleware := (&chain{}).Wrap
	routes := []plugins.Route{
		{
			Name:        "Create arc subscription",
			Methods:     []string{http.MethodPost},
			Path:        "/arc/subscription",
			HandlerFunc: px.postSubscription(),
			Description: "A proxy route to create ARC subscription.",
		},
		{
			Name:        "Post arc billing metadata",
			Methods:     []string{http.MethodPost},
			Path:        "/arc/metadata",
			HandlerFunc: px.postMetadata(),
			Description: "A proxy route to post metadata for an ARC subscription.",
		},
		{
			Name:        "Delete arc subscription",
			Methods:     []string{http.MethodDelete},
			Path:        "/arc/subscription",
			HandlerFunc: px.deleteSubscription(),
			Description: "A proxy route to delete ARC subscription.",
		},
		{
			Name:        "Get arc subscription",
			Methods:     []string{http.MethodGet},
			Path:        "/arc/instances",
			HandlerFunc: px.getSubscription(),
			Description: "A proxy route to get ARC subscription details.",
		},
		{
			Name:        "Get arc plan",
			Methods:     []string{http.MethodGet},
			Path:        "/arc/plan",
			HandlerFunc: px.getPlan(),
			Description: "A universal proxy route to get ARC plan details.",
		},
		{
			Name:        "Get arc plan from FS",
			Methods:     []string{http.MethodGet},
			Path:        "/arc/plan/fs",
			HandlerFunc: px.getPlanFS(),
			Description: "A universal proxy route to get ARC plan details from file system.",
		},
		{
			Name:        "Get curated insights",
			Methods:     []string{http.MethodGet},
			Path:        "/arc/curated_insights",
			HandlerFunc: px.getCuratedInsights(),
			Description: "A universal proxy route to get curated insights.",
		},
		{
			Name:        "Subscribe for curated insights",
			Methods:     []string{http.MethodPost},
			Path:        "/arc/curated_insights",
			HandlerFunc: px.subscribeCuratedInsights(),
			Description: "A universal proxy route to subscribe for curated insights.",
		},
		{
			Name:        "Unsubscribe for curated insights",
			Methods:     []string{http.MethodDelete},
			Path:        "/arc/curated_insights",
			HandlerFunc: px.unSubscribeCuratedInsights(),
			Description: "A universal proxy route to unsubscribe from curated insights.",
		},
		{
			Name:        "Update payment method",
			Methods:     []string{http.MethodPut},
			Path:        "/arc/payment",
			HandlerFunc: px.updatePaymentMethod(),
			Description: "A universal proxy route to update the payment method",
		},
	}
	return routes
}
