package permission

import (
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/op"
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
		category.Rules,
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
		DocsLimit:     10,
		SearchLimit:   10,
		IndicesLimit:  10,
		CatLimit:      10,
		ClustersLimit: 10,
		MiscLimit:     10,
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
