package user

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc"
)

var routes = []arc.Route{
	{
		Name:        "Get User",
		Methods:     []string{http.MethodGet},
		Path:        "/user",
		HandlerFunc: getUserHandler,
		Description: "Fetches the user object from the repository",
	},
	{
		Name:        "Put User",
		Methods:     []string{http.MethodPut},
		Path:        "/user",
		HandlerFunc: putUserHandler,
		Description: "Adds the user object to the repository",
	},
	{
		Name:        "Patch User",
		Methods:     []string{http.MethodPatch},
		Path:        "/user",
		HandlerFunc: patchUserHandler,
		Description: "Patches the user object with the passed fields in the repository",
	},
	{
		Name:        "Delete User",
		Methods:     []string{http.MethodDelete},
		Path:        "/user",
		HandlerFunc: deleteUserHandler,
		Description: "Deletes the user object from the repository",
	},
}
