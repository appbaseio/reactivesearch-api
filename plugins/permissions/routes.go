package permissions

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/route"
)

func (p *permissions) routes() []route.Route {
	middleware := (&chain{}).Wrap
	routes := []route.Route{
		{
			Name:        "Get permission",
			Methods:     []string{http.MethodGet},
			Path:        "/_permission/{username}",
			HandlerFunc: middleware(p.getPermission()),
			Description: "Returns the permission with {username}",
		},
		{
			Name:        "Create permission",
			Methods:     []string{http.MethodPost},
			Path:        "/_permission",
			HandlerFunc: middleware(p.postPermission()),
			Description: "Creates a new permission",
		},
		{
			Name:        "Patch permission",
			Methods:     []string{http.MethodPatch},
			Path:        "/_permission/{username}",
			HandlerFunc: middleware(p.patchPermission()),
			Description: "Updates the permission with {username}",
		},
		{
			Name:        "Delete permission",
			Methods:     []string{http.MethodDelete},
			Path:        "/_permission/{username}",
			HandlerFunc: middleware(p.deletePermission()),
			Description: "Deletes the permission with {username}",
		},
		{
			Name:        "Get user permissions",
			Methods:     []string{http.MethodGet},
			Path:        "/_permissions",
			HandlerFunc: middleware(p.getUserPermissions()),
			Description: "Returns all the permissions of the user",
		},
	}
	return routes
}
