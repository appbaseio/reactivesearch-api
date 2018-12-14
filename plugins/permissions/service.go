package permissions

import "github.com/appbaseio-confidential/arc/model/permission"

type permissionService interface {
	getPermission(username string) (*permission.Permission, error)
	getRawPermission(username string) ([]byte, error)
	postPermission(p permission.Permission) (bool, error)
	patchPermission(username string, patch map[string]interface{}) ([]byte, error)
	deletePermission(username string) (bool, error)
	getRawOwnerPermissions(owner string) ([]byte, error)
}
