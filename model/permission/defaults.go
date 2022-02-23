package permission

import (
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/op"
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
		category.Suggestions,
		category.Auth,
		category.ReactiveSearch,
		category.SearchRelevancy,
		category.Synonyms,
		category.SearchGrader,
		category.UIBuilder,
		category.Logs,
		category.Cache,
		category.StoredQuery,
		category.Sync,
		category.Pipelines,
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
		IPLimit:               7200,
		DocsLimit:             10,
		SearchLimit:           10,
		IndicesLimit:          10,
		CatLimit:              10,
		ClustersLimit:         10,
		MiscLimit:             10,
		UserLimit:             10,
		PermissionLimit:       10,
		AnalyticsLimit:        10,
		RulesLimit:            10,
		SuggestionsLimit:      10,
		StreamsLimit:          10,
		AuthLimit:             10,
		ReactiveSearchLimit:   10,
		SearchRelevancyLimit:  10,
		SearchGraderLimit:     10,
		EcommIntegrationLimit: 10,
		LogsLimit:             10,
		SynonymsLimit:         10,
		CacheLimit:            10,
		StoredQueryLimit:      10,
		SyncLimit:             10,
		PipelinesLimit:        10,
	}

	defaultAdminLimits = Limits{
		IPLimit:               7200,
		DocsLimit:             30,
		SearchLimit:           30,
		IndicesLimit:          30,
		CatLimit:              30,
		ClustersLimit:         30,
		MiscLimit:             30,
		UserLimit:             30,
		PermissionLimit:       30,
		AnalyticsLimit:        30,
		RulesLimit:            30,
		SuggestionsLimit:      30,
		StreamsLimit:          30,
		AuthLimit:             30,
		ReactiveSearchLimit:   30,
		SearchRelevancyLimit:  30,
		SearchGraderLimit:     30,
		EcommIntegrationLimit: 30,
		LogsLimit:             30,
		SynonymsLimit:         30,
		CacheLimit:            30,
		StoredQueryLimit:      30,
		SyncLimit:             30,
		PipelinesLimit:        30,
	}
)
