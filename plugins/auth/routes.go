package auth

import (
	"net/http"

	"github.com/appbaseio/arc/plugins"
)

func (a *Auth) routes() []plugins.Route {
	routes := []plugins.Route{
		{
			Name:        "Get public key",
			Methods:     []string{http.MethodGet},
			Path:        "/_public_key",
			HandlerFunc: a.getPublicKey(),
			Description: "GET the public key",
		},
		{
			Name:        "Put public key",
			Methods:     []string{http.MethodPut},
			Path:        "/_public_key",
			HandlerFunc: a.setPublicKey(),
			Description: "Create or Update the public key",
		},
	}
	return routes
}
