package users

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/model/user"
)

type userService interface {
	getRawUsers(ctx context.Context) ([]byte, error)
	getUser(ctx context.Context, username string) (*user.User, error)
	getRawUser(ctx context.Context, username string) ([]byte, error)
	postUser(ctx context.Context, u user.User) (bool, error)
	patchUser(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error)
	deleteUser(ctx context.Context, username string) (bool, error)
}
