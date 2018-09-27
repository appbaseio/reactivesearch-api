package permissions

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (p *Permissions) routes() []plugin.Route {
	var routes = []plugin.Route{
		{
			Name:        "Get Permission",
			Methods:     []string{http.MethodGet},
			Path:        "/permission/{username}",
			HandlerFunc: opClassifier(p.getPermission()),
			Description: "Fetch the permission object from the repository",
		},
		{
			Name:        "Create Permission",
			Methods:     []string{http.MethodPut},
			Path:        "/permission",
			HandlerFunc: opClassifier(p.putPermission()),
			Description: "Create a new permission object in the repository",
		},
		{
			Name:        "Patch Permission",
			Methods:     []string{http.MethodPatch},
			Path:        "/permission/{username}",
			HandlerFunc: opClassifier(p.patchPermission()),
			Description: "Update the permission object in the repository",
		},
		{
			Name:        "Delete Permission",
			Methods:     []string{http.MethodDelete},
			Path:        "/permission/{username}",
			HandlerFunc: opClassifier(p.deletePermission()),
			Description: "Delete the permission object in the repository",
		},
	}
	return routes
}
