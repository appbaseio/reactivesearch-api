package permission

import (
	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
)

var (
	defaultCategories = []category.Category{
		category.Docs,
		category.Search,
		category.Indices,
	}

	adminCategories = []category.Category{
		category.Docs,
		category.Search,
		category.Indices,
		category.Cat,
		category.Clusters,
		category.Misc,
		category.User,
		category.Permission,
		category.Analytics,
		category.Streams,
	}

	defaultOps = []op.Operation{
		op.Read,
	}

	adminOps = []op.Operation{
		op.Read,
		op.Write,
		op.Delete,
	}

	defaultLimits = Limits{
		IPLimit:       7200,
		DocsLimit:     5,
		SearchLimit:   5,
		IndicesLimit:  5,
		CatLimit:      5,
		ClustersLimit: 5,
		MiscLimit:     5,
	}

	defaultAdminLimits = Limits{
		IPLimit:       7200,
		DocsLimit:     30,
		SearchLimit:   30,
		IndicesLimit:  30,
		CatLimit:      30,
		ClustersLimit: 30,
		MiscLimit:     30,
	}
)
