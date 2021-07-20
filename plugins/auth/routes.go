package auth

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (a *Auth) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	routes := []plugins.Route{
		{
			Name:        "Get public key",
			Methods:     []string{http.MethodGet},
			Path:        "/_public_key",
			HandlerFunc: middleware(a.getPublicKey()),
			Description: "GET the public key",
		},
		{
			Name:        "Put public key",
			Methods:     []string{http.MethodPut},
			Path:        "/_public_key",
			HandlerFunc: middleware(a.setPublicKey()),
			Description: "Create or Update the public key",
		},
	}
	return routes
}
