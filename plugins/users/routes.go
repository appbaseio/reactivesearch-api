package users

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
)

func (u *users) routes() []plugin.Route {
	middleware := (&chain{}).Wrap
	routes := []plugin.Route{
		{
			Name:        "Get user",
			Methods:     []string{http.MethodGet},
			Path:        "/_user",
			HandlerFunc: middleware(u.getUser()),
			Description: "Fetches the user from the repository",
		},
		{
			Name:        "Get another user",
			Methods:     []string{http.MethodGet},
			Path:        "/_user/{user_id}",
			HandlerFunc: middleware(isAdmin(u.getUserWithId())),
			Description: "Fetches another user from the repository",
		},
		{
			Name:        "Put user",
			Methods:     []string{http.MethodPut},
			Path:        "/_user",
			HandlerFunc: middleware(isAdmin(u.putUser())),
			Description: "Adds the user to the repository",
		},
		{
			Name:        "Patch user",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user",
			HandlerFunc: middleware(u.patchUser()),
			Description: "Patches the user with the passed fields in the repository",
		},
		{
			Name:        "Patch another user",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user/{user_id}",
			HandlerFunc: middleware(isAdmin(u.patchUserWithId())),
			Description: "Patches another user with passed fields in the repository",
		},
		{
			Name:        "Delete user",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user",
			HandlerFunc: middleware(u.deleteUser()),
			Description: "Deletes the user from the repository",
		},
		{
			Name:        "Delete another user",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user/{user_id}",
			HandlerFunc: middleware(isAdmin(u.deleteUserWithId())),
			Description: "Deletes another user from the repository",
		},
	}
	return routes
}
