package permission

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

	defaultLimits = Limits{
		IPLimit:          7200,
		DocsLimit:        5,
		SearchLimit:      5,
		IndicesLimit:     5,
		CatLimit:         5,
		ClustersLimit:    5,
		MiscLimit:        5,
		UsersLimit:       5,
		PermissionsLimit: 5,
		AnalyticsLimit:   5,
		StreamsLimit:     5,
	}

	defaultAdminLimits = Limits{
		IPLimit:          7200,
		DocsLimit:        30,
		SearchLimit:      30,
		IndicesLimit:     30,
		CatLimit:         30,
		ClustersLimit:    30,
		MiscLimit:        30,
		UsersLimit:       30,
		PermissionsLimit: 30,
		AnalyticsLimit:   30,
		StreamsLimit:     30,
	}
)
