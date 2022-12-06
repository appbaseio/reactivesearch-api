package users

import (
	"context"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/model/user"
)

type userService interface {
	getRawUsers(ctx context.Context, req *http.Request) ([]byte, error)
	getUser(ctx context.Context, req *http.Request, username string) (*user.User, error)
	getRawUser(ctx context.Context, req *http.Request, username string) ([]byte, error)
	postUser(ctx context.Context, req *http.Request, u user.User) (bool, error)
	patchUser(ctx context.Context, req *http.Request, username string, patch map[string]interface{}) ([]byte, error)
	deleteUser(ctx context.Context, req *http.Request, username string) (bool, error)
	getUserID(ctx context.Context, req *http.Request, username string) (string, error)
}
