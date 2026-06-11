package cache

import (
	"os"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

const (
	logTag              = "[cache]"
	defaultCacheEsIndex = ".cache"
	envCacheEsIndex     = "CACHE_ES_INDEX"
	typeName            = "_doc"
	cacheConfigDocID    = "cache_preferences"
	mapping             = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
)

var (
	singleton *Cache
	once      sync.Once
)

type Cache struct {
	es          cacheService
	cacheConfig CacheConfig
}

// Name returns the name of the plugin: "[cache]"
func (f *Cache) Name() string {
	return logTag
}

// Instance returns the singleton instace of Cache plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Cache in order to avoid stateless instances of the plugin.
func Instance() *Cache {
	once.Do(func() { singleton = &Cache{} })
	return singleton
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (c *Cache) InitFunc() error {
	cacheIndex := os.Getenv(envCacheEsIndex)
	if cacheIndex == "" {
		cacheIndex = defaultCacheEsIndex
	}
	cacheIndex = util.MetaIndexName(cacheIndex)
	// initialize the dao
	var err error
	c.es, err = initPlugin(cacheIndex, mapping)
	if err != nil {
		return err
	}
	cacheConfig, err := c.es.populateDefaultConfig()
	if err != nil {
		log.Errorln(logTag, "error encountered while initing with default cache preferences", err)
		return err
	}
	if cacheConfig != nil && cacheConfig.MaxSize != nil {
		c.cacheConfig = *cacheConfig
		setCachePreferences(c.cacheConfig)
	}

	// Set plugin cache sync script
	script := CacheSyncScript{
		index: cacheIndex,
	}
	util.AddSyncScript(script)

	// Initialize the search cache
	initializeSearchCache()

	return nil
}

func (c *Cache) Routes() []plugins.Route {
	return c.routes()
}

func (c *Cache) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

func (c *Cache) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		saveToCacheRSAPI,
	}
}

// Expose plugin specific routes
func (c *Cache) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
