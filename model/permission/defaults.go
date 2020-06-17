package permission

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
		category.Auth,
		category.Functions,
		category.ReactiveSearch,
		category.SearchRelevancy,
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

	defaultLimits = Limits{
		IPLimit:              7200,
		DocsLimit:            10,
		SearchLimit:          10,
		IndicesLimit:         10,
		CatLimit:             10,
		ClustersLimit:        10,
		MiscLimit:            10,
		UserLimit:            10,
		PermissionLimit:      10,
		AnalyticsLimit:       10,
		RulesLimit:           10,
		TemplatesLimit:       10,
		SuggestionsLimit:     10,
		StreamsLimit:         10,
		AuthLimit:            10,
		FunctionsLimit:       10,
		ReactiveSearchLimit:  10,
		SearchRelevancyLimit: 10,
		SearchGraderLimit:    10,
	}

	defaultAdminLimits = Limits{
		IPLimit:              7200,
		DocsLimit:            30,
		SearchLimit:          30,
		IndicesLimit:         30,
		CatLimit:             30,
		ClustersLimit:        30,
		MiscLimit:            30,
		UserLimit:            30,
		PermissionLimit:      30,
		AnalyticsLimit:       30,
		RulesLimit:           30,
		TemplatesLimit:       30,
		SuggestionsLimit:     30,
		StreamsLimit:         30,
		AuthLimit:            30,
		FunctionsLimit:       30,
		ReactiveSearchLimit:  30,
		SearchRelevancyLimit: 30,
		SearchGraderLimit:    30,
	}
)
