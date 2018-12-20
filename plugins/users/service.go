package users

import "github.com/appbaseio-confidential/arc/model/user"

type userService interface {
	getUser(username string) (*user.User, error)
	getRawUser(username string) ([]byte, error)
	postUser(u user.User) (bool, error)
	patchUser(username string, patch map[string]interface{}) ([]byte, error)
	deleteUser(username string) (bool, error)
}
