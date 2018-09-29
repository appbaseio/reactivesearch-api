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

	defaultOps = []op.Operation{
		op.Read,
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
)
