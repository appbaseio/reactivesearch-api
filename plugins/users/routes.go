package users

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/plugins/auth"
)

// Middleware chain:
// r -> classifier -> basicAuth -> validateOp -> validateACL -> rateLimit
//
// 1. classifier:
// 2. basicAuth:
// 3. validateOp:
// 4. validateACL:
// 5. isAdmin (optional)
// 6. rateLimit:
func (u *Users) routes() []plugin.Route {
	// TODO: plugin dependency
	var basicAuth = auth.Instance().BasicAuth
	var routes = []plugin.Route{
		{
			Name:        "Get User",
			Methods:     []string{http.MethodGet},
			Path:        "/user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.getUser())))),
			Description: "Fetches the user object from the repository",
		},
		{
			Name:        "Put User",
			Methods:     []string{http.MethodPut},
			Path:        "/user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.putUser())))),
			Description: "Adds the user object to the repository",
		},
		{
			Name:        "Patch User",
			Methods:     []string{http.MethodPatch},
			Path:        "/user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.patchUser())))),
			Description: "Patches the user object with the passed fields in the repository",
		},
		{
			Name:        "Delete User",
			Methods:     []string{http.MethodDelete},
			Path:        "/user",
			HandlerFunc: classifier(basicAuth(validateOp(validateACL(u.deleteUser())))),
			Description: "Deletes the user object from the repository",
		},
	}
	return routes
}
