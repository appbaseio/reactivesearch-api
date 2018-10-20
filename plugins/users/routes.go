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
			Description: "Returns the user",
		},
		{
			Name:        "Get user with {user_id}",
			Methods:     []string{http.MethodGet},
			Path:        "/_user/{user_id}",
			HandlerFunc: middleware(isAdmin(u.getUserWithID())),
			Description: "Returns the user with {user_id}",
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
			Name:        "Patch user with {user_id}",
			Methods:     []string{http.MethodPatch},
			Path:        "/_user/{user_id}",
			HandlerFunc: middleware(isAdmin(u.patchUserWithID())),
			Description: "Modifies the user with {user_id}",
		},
		{
			Name:        "Delete user",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user",
			HandlerFunc: middleware(u.deleteUser()),
			Description: "Deletes the user",
		},
		{
			Name:        "Delete user with {user_id}",
			Methods:     []string{http.MethodDelete},
			Path:        "/_user/{user_id}",
			HandlerFunc: middleware(isAdmin(u.deleteUserWithID())),
			Description: "Deletes the user with {user_id}",
		},
	}
	return routes
}
