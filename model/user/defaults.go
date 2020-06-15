package user

import (
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
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
		category.Templates,
		category.Suggestions,
		category.Functions,
		category.ReactiveSearch,
		category.SearchRelevancy,
		category.Auth,
		category.Synonyms,
		category.SearchGrader,
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
