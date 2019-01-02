package analytics

import "context"

type analyticsService interface {
	analyticsOverview(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error)
	advancedAnalytics(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error)
	popularSearches(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) (interface{}, error)
	popularSearchesRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error)
	noResultSearches(ctx context.Context, from, to string, size int, indices ...string) (interface{}, error)
	noResultSearchesRaw(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error)
	popularFilters(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) (interface{}, error)
	popularFiltersRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error)
	popularResults(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) (interface{}, error)
	popularResultsRaw(ctx context.Context, from, to string, size int, clickAnalytics bool, indices ...string) ([]byte, error)
	geoRequestsDistribution(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error)
	latencies(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error)
	summary(ctx context.Context, from, to string, indices ...string) ([]byte, error)
	totalSearches(ctx context.Context, from, to string, indices ...string) (float64, error)
	totalConversions(ctx context.Context, from, to string, indices ...string) (float64, error)
	totalClicks(ctx context.Context, from, to string, indices ...string) (float64, error)
	searchHistogram(ctx context.Context, from, to string, size int, indices ...string) (interface{}, error)
	searchHistogramRaw(ctx context.Context, from, to string, size int, indices ...string) ([]byte, error)
	getRequestDistribution(ctx context.Context, from, to, interval string, size int, indices ...string) ([]byte, error)
	indexRecord(ctx context.Context, docID string, record map[string]interface{})
	updateRecord(ctx context.Context, docID string, record map[string]interface{})
	deleteOldRecords()
}
