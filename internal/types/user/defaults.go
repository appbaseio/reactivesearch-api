package user

import (
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
)

var (
	defaultACLs = []acl.ACL{
		acl.Docs,
		acl.Search,
	}

	adminACLs = []acl.ACL{
		acl.Docs,
		acl.Search,
		acl.Indices,
		acl.Cat,
		acl.Clusters,
		acl.Misc,
		acl.User,
		acl.Permission,
		acl.Analytics,
		acl.Streams,
	}

	defaultCategories = []category.Category{}

	adminCategories = []category.Category{}

	defaultOps = []op.Operation{
		op.Read,
	}

	adminOps = []op.Operation{
		op.Read,
		op.Write,
		op.Delete,
	}

	// NOTE: we are storing the address of the isAdmin variable in the user
	isAdminTrue  = true
	isAdminFalse = false
)
