package analytics

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/plugins/analyticsrequest"
)

type analyticsService interface {
	analyticsOverview(ctx context.Context, queryParams QueryParams, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error)
	advancedAnalytics(ctx context.Context, queryParams QueryParams, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error)
	summary(ctx context.Context, from, to string, filters map[string]interface{}, indices ...string) ([]byte, error)
	getFilterLabels(ctx context.Context) ([]byte, error)
	getFilterValues(ctx context.Context, label, prefix string, indices ...string) ([]byte, error)
	updateRecord(ctx context.Context, docID string, record analyticsrequest.Record) *Error
	deleteOldRecords()
	updateUserSession(ctx context.Context, docID string, record UserSession, customEvents map[string]interface{}, queryID string) error
	getActiveUserSessions(ctx context.Context) ([]ActiveUserSessionES, error)
	updateInsightStatus(ctx context.Context, docID string, insightID InsightType, insightStatus InsightStatusRequest) error
	getInsightStatus(ctx context.Context, docID string) (InsightStatusEsDoc, error)
	rolloverIndexJob(alias string)
	getInsights(ctx context.Context, indexName string, disableFilterByStatus bool) (GetInsight, error)
	getSortedIndices(aliasName string) []string
	reportAnalyticsToUsers()
	queryOverview(ctx context.Context, queryParams QueryParams, query string, filters map[string]interface{}, indices ...string) ([]byte, error)
	storedQueriesUsage(ctx context.Context, queryParams QueryParams, filters map[string]interface{}) ([]byte, error)
	queryRulesUsage(ctx context.Context, queryParams QueryParams, filters map[string]interface{}) ([]byte, error)
	recentSearches(ctx context.Context, from, to string, size int, minChar *int, filters map[string]interface{}, indices ...string) ([]byte, error)
	popularSearchesWithSummary(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error)
	noResultSearchesWithSummary(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error)
	popularFiltersWithSummary(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error)
	popularResultsWithSummary(ctx context.Context, from, to string, size int, clickAnalytics bool, filters map[string]interface{}, indices ...string) ([]byte, error)
	recentResults(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error)
	geoRequestsDistributionWithSummary(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error)
	getRequestDistributionWithSummary(ctx context.Context, queryParams QueryParams, interval string, size int, filters map[string]interface{}, indices ...string) ([]byte, error)
	latenciesWithSummary(ctx context.Context, from, to string, size int, filters map[string]interface{}, indices ...string) ([]byte, error)
	deleteOldMetricBeatIndices()
	updateConversion(ctx context.Context, docID string, record analyticsrequest.Record) *Error
	updateSavedSearch(ctx context.Context, record SavedSearchRequest) *Error
	getSavedSearches(ctx context.Context, filters SavedSearchesFilters) ([]SavedSearchES, error)
	updateFavorite(ctx context.Context, record FavoriteRequest) *Error
	getFavorites(ctx context.Context, filters SavedSearchesFilters) ([]map[string]interface{}, error)
	savePreferences(ctx context.Context, preferences AnalyticsPreferences) error
	getPreferences(ctx context.Context) (AnalyticsPreferences, error)
}

type recentDocumentsService interface {
	createRecentDocument(ctx context.Context, record RecentDocument, docId string) error
	updateRecentDocumentWithUserId(ctx context.Context, docId string, userId string) error
	doesDocumentExist(ctx context.Context, docId string) (bool, error)
	getRecentDocumentsWithFilter(ctx context.Context, params QueryParams, userId string, indicesPassed []string) ([]byte, error)
}
