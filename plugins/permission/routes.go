package permission

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc"
)

var routes = []arc.Route{
	{
		Name:        "Get Permission",
		Methods:     []string{http.MethodGet},
		Path:        "/permission/{username}",
		HandlerFunc: getPermissionHandler,
		Description: "Fetch the permission object from the repository",
	},
	{
		Name:        "Create Permission",
		Methods:     []string{http.MethodPut},
		Path:        "/permission",
		HandlerFunc: putPermissionHandler,
		Description: "Create a new permission object in the repository",
	},
	{
		Name:        "Patch Permission",
		Methods:     []string{http.MethodPatch},
		Path:        "/permission/{username}",
		HandlerFunc: patchPermissionHandler,
		Description: "Update the permission object in the repository",
	},
	{
		Name:        "Delete Permission",
		Methods:     []string{http.MethodDelete},
		Path:        "/permission/{username}",
		HandlerFunc: deletePermissionHandler,
		Description: "Delete the permission object in the repository",
	},
}
