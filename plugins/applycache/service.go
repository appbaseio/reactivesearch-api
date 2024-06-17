package applycache

import "context"

type cacheService interface {
	addStat(ctx context.Context, statRecord CacheStatES) error
	updateStatScript(ctx context.Context, scriptString string, params map[string]interface{}, dayID string) error
	deleteOlderRecordsByDate(ctx context.Context, maxTime int64) error
	isIdExists(ctx context.Context, id string) (bool, error)
	getCacheAnalytics(ctx context.Context, from string, to string) ([]byte, error)
}
