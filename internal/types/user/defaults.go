package user

import (
	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
)

var (
	defaultCategories = []category.Category{
		category.Docs,
		category.Search,
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

	// NOTE: we are storing the address of the isAdmin variable in the user
	isAdminTrue  = true
	isAdminFalse = false
)
