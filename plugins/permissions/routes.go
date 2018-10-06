package permissions

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/plugins/auth"
)

func (p *permissions) routes() []plugin.Route {
	basicAuth := auth.Instance().BasicAuth
	routes := []plugin.Route{
		{
			Name:        "Get Permission",
			Methods:     []string{http.MethodGet},
			Path:        "/_permission/{username}",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(p.getPermission())))),
			Description: "Fetch the permission object from the repository",
		},
		{
			Name:        "Create Permission",
			Methods:     []string{http.MethodPut},
			Path:        "/_permission",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(p.putPermission())))),
			Description: "Create a new permission object in the repository",
		},
		{
			Name:        "Patch Permission",
			Methods:     []string{http.MethodPatch},
			Path:        "/_permission/{username}",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(p.patchPermission())))),
			Description: "Update the permission object in the repository",
		},
		{
			Name:        "Delete Permission",
			Methods:     []string{http.MethodDelete},
			Path:        "/_permission/{username}",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(p.deletePermission())))),
			Description: "Delete the permission object in the repository",
		},
	}
	return routes
}
