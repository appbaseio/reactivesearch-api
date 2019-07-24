package permissions

import (
	"context"
	"github.com/appbaseio/arc/model/permission"
)

type permissionService interface {
	getPermission(ctx context.Context, username string) (*permission.Permission, error)
	getRawPermission(ctx context.Context, username string) ([]byte, error)
	postPermission(ctx context.Context, p permission.Permission) (bool, error)
	patchPermission(ctx context.Context, username string, patch map[string]interface{}) ([]byte, error)
	deletePermission(ctx context.Context, username string) (bool, error)
	getRawOwnerPermissions(ctx context.Context, owner string) ([]byte, error)
	getRawRolePermission(ctx context.Context, role string) ([]byte, error)
	checkRoleExists(ctx context.Context, role string) (bool, error)
}
