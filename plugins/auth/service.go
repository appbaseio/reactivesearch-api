package auth

import (
	"context"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
)

type authService interface {
	getCredential(ctx context.Context, username, password string) (interface{}, error)
	putUser(ctx context.Context, u user.User) (bool, error)
	getUser(ctx context.Context, username string) (*user.User, error)
	getRawUser(ctx context.Context, username string) ([]byte, error)
	putPermission(ctx context.Context, p permission.Permission) (bool, error)
	getPermission(ctx context.Context, username string) (*permission.Permission, error)
	getRawPermission(ctx context.Context, username string) ([]byte, error)
}
