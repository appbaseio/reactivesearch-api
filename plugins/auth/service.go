package auth

import (
	"context"

	"github.com/appbaseio-confidential/reactivesearch/model/credential"
	"github.com/appbaseio-confidential/reactivesearch/model/permission"
	"github.com/appbaseio-confidential/reactivesearch/model/user"
)

type authService interface {
	getCredential(ctx context.Context, username string) (credential.AuthCredential, error)
	putUser(ctx context.Context, u user.User) (bool, error)
	getUser(ctx context.Context, username string) (*user.User, error)
	getRawUser(ctx context.Context, username string) ([]byte, error)
	putPermission(ctx context.Context, p permission.Permission) (bool, error)
	getPermission(ctx context.Context, username string) (*permission.Permission, error)
	getRawPermission(ctx context.Context, username string) ([]byte, error)
	getRolePermission(ctx context.Context, role string) (*permission.Permission, error)
	createIndex(indexName, mapping string) (bool, error)
	savePublicKey(ctx context.Context, indexName string, record publicKey) (interface{}, error)
	getPublicKey(ctx context.Context) (publicKey, error)
}
