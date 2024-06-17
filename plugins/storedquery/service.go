package storedquery

import "context"

type storedQueryService interface {
	getStoredQueries(ctx context.Context) ([]ESStoredQueryDOC, error)
	updateStoredQuery(ctx context.Context, queryID string, record ESStoredQueryDOC) error
	deleteStoredQuery(ctx context.Context, storedQuery string) error
	validateQuery(ctx context.Context, q string) (*bool, error)
	executeQuery(ctx context.Context, index, query string) ([]byte, error)
}
