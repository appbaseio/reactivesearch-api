package permissions

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (p *permissions) routes() []plugin.Route {
	middleware := (&chain{}).Wrap
	routes := []plugin.Route{
		{
			Name:        "Get permission",
			Methods:     []string{http.MethodGet},
			Path:        "/_permission/{username}",
			HandlerFunc: middleware(p.getPermission()),
			Description: "Returns the permission from the repository",
		},
		{
			Name:        "Create permission",
			Methods:     []string{http.MethodPost},
			Path:        "/_permission",
			HandlerFunc: middleware(p.postPermission()),
			Description: "Create a new permission in the repository",
		},
		{
			Name:        "Patch permission",
			Methods:     []string{http.MethodPatch},
			Path:        "/_permission/{username}",
			HandlerFunc: middleware(p.patchPermission()),
			Description: "Update the permission in the repository",
		},
		{
			Name:        "Delete permission",
			Methods:     []string{http.MethodDelete},
			Path:        "/_permission/{username}",
			HandlerFunc: middleware(p.deletePermission()),
			Description: "Delete the permission in the repository",
		},
		{
			Name:        "Get user permissions",
			Methods:     []string{http.MethodGet},
			Path:        "/_permissions",
			HandlerFunc: middleware(p.getUserPermissions()),
			Description: "Returns all the permissions associated with user",
		},
	}
	return routes
}
