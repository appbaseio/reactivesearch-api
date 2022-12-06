package auth

import (
	"context"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
)

type authService interface {
	getCredential(ctx context.Context, req *http.Request, username string) (credential.AuthCredential, error)
	putUser(ctx context.Context, req *http.Request, u user.User) (bool, error)
	getUser(ctx context.Context, req *http.Request, username string) (*user.User, error)
	getRawUser(ctx context.Context, req *http.Request, username string) ([]byte, error)
	putPermission(ctx context.Context, req *http.Request, p permission.Permission) (bool, error)
	getPermission(ctx context.Context, req *http.Request, username string) (*permission.Permission, error)
	getRawPermission(ctx context.Context, req *http.Request, username string) ([]byte, error)
	getRolePermission(ctx context.Context, req *http.Request, role string) (*permission.Permission, error)
	createIndex(indexName, mapping string) (bool, error)
	savePublicKey(ctx context.Context, req *http.Request, indexName string, record publicKey) (interface{}, error)
	getPublicKey(ctx context.Context, req *http.Request) (publicKey, error)
}
