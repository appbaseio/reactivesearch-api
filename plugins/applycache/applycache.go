package applycache

import (
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
)

const (
	logTag       = "[applycache]"
	cacheEsIndex = ".cache_stats"
	typeName     = "_doc"
	mapping      = `{"mappings": %s, "settings":{ %s "index.number_of_shards":1,"index.number_of_replicas":%d}}`
)

var (
	singleton *Cache
	once      sync.Once
)

type Cache struct {
	es       cacheService
	rollOver *RollOverStat
}

const cacheMapping = `{}`

// Name returns the name of the plugin: "[applycache]"
func (f *Cache) Name() string {
	return logTag
}

// Instance returns the singleton instance of Cache plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Cache in order to avoid stateless instances of the plugin.
func Instance() *Cache {
	once.Do(func() { singleton = &Cache{} })
	return singleton
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (c *Cache) InitFunc() error {
	indexPrefix := util.MetaIndexName(cacheEsIndex)

	// initialize the dao
	var err error
	c.es, err = initPlugin(indexPrefix, mapping)
	if err != nil {
		return err
	}

	// Init the rollOver variable
	rollOverInstance := new(RollOverStat)
	c.rollOver = rollOverInstance.Init()

	// Start the jobs
	c.StartJobs()

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
		trackCachePlugin,
		applyCachedResponse,
	}
}

// Expose plugin specific routes
func (c *Cache) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
