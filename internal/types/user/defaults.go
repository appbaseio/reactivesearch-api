package user

import (
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/op"
)

var (
	defaultACLs = []acl.ACL{
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

	defaultAdminACLs = []acl.ACL{
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

	defaultOps = []op.Operation{
		op.Read,
	}

	defaultAdminOps = []op.Operation{
		op.Read,
		op.Write,
		op.Delete,
	}

	isAdminTrue = true
	isAdminFalse = false
)
