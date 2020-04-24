package permissions

import (
	"net/http"

	"github.com/appbaseio/arc/plugins"
)

func (p *permissions) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	routes := []plugins.Route{
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
			Path:        "/user/_permissions",
			HandlerFunc: middleware(p.getUserPermissions()),
			Description: "Returns all the permissions of the user",
		},
		{
			Name:        "Get permissions",
			Methods:     []string{http.MethodGet},
			Path:        "/_permissions",
			HandlerFunc: middleware(p.getPermissions()),
			Description: "Returns all the permissions of the cluster",
		},
		{
			Name:        "Get permissions",
			Methods:     []string{http.MethodGet},
			Path:        "/{index}/_permissions",
			HandlerFunc: middleware(p.getPermissions()),
			Description: "Returns all the permissions for a particular index",
		},
		{
			Name:        "Create/Read/Update/Delete permission by role",
			Methods:     []string{http.MethodPost, http.MethodGet, http.MethodPatch, http.MethodDelete},
			Path:        "/_role/{name}",
			HandlerFunc: middleware(p.role()),
			Description: "CRUD the permission with role {name}",
		},
	}
	return routes
}
