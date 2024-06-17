package cache

import "context"

type cacheService interface {
	savePreferences(ctx context.Context, cacheConfig CacheConfig) error
	getPreferences(ctx context.Context) (CacheConfig, error)
	populateDefaultConfig() (*CacheConfig, error)
	clearCache()
}
