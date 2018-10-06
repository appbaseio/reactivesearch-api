package users

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/plugins/auth"
)

func (u *users) routes() []plugin.Route {
	basicAuth := auth.Instance().BasicAuth
	routes := []plugin.Route{
		{
			Name:        "Get user",
			Methods:     []string{http.MethodGet},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.getUser())))),
			Description: "Fetches the user from the repository",
		},
		{
			Name:        "Get another user",
			Methods:     []string{http.MethodGet},
			Path:        "/_user/{user_id}",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(isAdmin(u.getAnotherUser()))))),
			Description: "Fetches another user from the repository",
		},
		{
			Name:        "Put user",
			Methods:     []string{http.MethodPut},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(isAdmin(u.putUser()))))),
			Description: "Adds the user to the repository",
		},
		{
			Name:        "Patch user",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.patchUser())))),
			Description: "Patches the user with the passed fields in the repository",
		},
		{
			Name:        "Patch another user",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user/{user_id}",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(isAdmin(u.patchAnotherUser()))))),
			Description: "Patches another user with passed fields in the repository",
		},
		{
			Name:        "Delete user",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.deleteUser())))),
			Description: "Deletes the user from the repository",
		},
		{
			Name:        "Delete another user",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user/{user_id}",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(isAdmin(u.deleteAnotherUser()))))),
			Description: "Deletes another user from the repository",
		},
	}
	return routes
}
