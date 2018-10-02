package users

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/plugins/auth"
)

func (u *Users) routes() []plugin.Route {
	basicAuth := auth.New().BasicAuth
	routes := []plugin.Route{
		{
			Name:        "Get User",
			Methods:     []string{http.MethodGet},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.getUser())))),
			Description: "Fetches the user object from the repository",
		},
		{
			Name:        "Put User",
			Methods:     []string{http.MethodPut},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(isAdmin(u.putUser()))))),
			Description: "Adds the user object to the repository",
		},
		{
			Name:        "Patch User",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(isAdmin(u.patchUser()))))),
			Description: "Patches the user object with the passed fields in the repository",
		},
		{
			Name:        "Delete User",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(isAdmin(u.deleteUser()))))),
			Description: "Deletes the user object from the repository",
		},
	}
	return routes
}
