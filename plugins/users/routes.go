package users

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (u *Users) routes() []plugin.Route {
	var routes = []plugin.Route{
		{
			Name:        "Get User",
			Methods:     []string{http.MethodGet},
			Path:        "/user",
			HandlerFunc: u.getUser(),
			Description: "Fetches the user object from the repository",
		},
		{
			Name:        "Put User",
			Methods:     []string{http.MethodPut},
			Path:        "/user",
			HandlerFunc: u.putUser(),
			Description: "Adds the user object to the repository",
		},
		{
			Name:        "Patch User",
			Methods:     []string{http.MethodPatch},
			Path:        "/user",
			HandlerFunc: u.patchUser(),
			Description: "Patches the user object with the passed fields in the repository",
		},
		{
			Name:        "Delete User",
			Methods:     []string{http.MethodDelete},
			Path:        "/user",
			HandlerFunc: u.deleteUser(),
			Description: "Deletes the user object from the repository",
		},
	}
	return routes
}
