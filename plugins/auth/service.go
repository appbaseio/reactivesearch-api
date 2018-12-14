package auth

import (
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
)

type authService interface {
	getCredential(username, password string) (interface{}, error)
	putUser(u user.User) (bool, error)
	getUser(username string) (*user.User, error)
	getRawUser(username string) ([]byte, error)
	putPermission(p permission.Permission) (bool, error)
	getPermission(username string) (*permission.Permission, error)
	getRawPermission(username string) ([]byte, error)
}