package users

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/plugins"
)

func (u *Users) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	routes := []plugins.Route{
		{
			Name:        "Get user",
			Methods:     []string{http.MethodGet},
			Path:        "/_user",
			HandlerFunc: middleware(u.getUser()),
			Description: "Returns the user",
		},
		{
			Name:        "Get user with {username}",
			Methods:     []string{http.MethodGet},
			Path:        "/_user/{username}",
			HandlerFunc: middleware(isAdmin(u.getUserWithUsername())),
			Description: "Returns the user with {username}",
		},
		{
			Name:        "Get all users",
			Methods:     []string{http.MethodGet},
			Path:        "/_users",
			HandlerFunc: middleware(isAdmin(u.getAllUsers())),
			Description: "Returns all the users",
		},
		{
			Name:        "Post user",
			Methods:     []string{http.MethodPost},
			Path:        "/_user",
			HandlerFunc: middleware(isAdmin(u.postUser())),
			Description: "Creates a new user",
		},
		{
			Name:        "Patch user",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user",
			HandlerFunc: middleware(u.patchUser()),
			Description: "Modifies the user",
		},
		{
			Name:        "Patch user with {username}",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user/{username}",
			HandlerFunc: middleware(isAdmin(u.patchUserWithUsername())),
			Description: "Modifies the user with {username}",
		},
		{
			Name:        "Delete user",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user",
			HandlerFunc: middleware(u.deleteUser()),
			Description: "Deletes the user",
		},
		{
			Name:        "Delete user with {username}",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user/{username}",
			HandlerFunc: middleware(isAdmin(u.deleteUserWithUsername())),
			Description: "Deletes the user with {username}",
		},
	}
	return routes
}
