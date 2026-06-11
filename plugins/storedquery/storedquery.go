package storedquery

import (
	"context"
	"os"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

const (
	logTag                    = "[storedquery]"
	defaultStoredQueryEsIndex = ".storedquery"
	typeName                  = "_doc"
	envStoredQueryEsIndex     = "STORED_QUERY_ES_INDEX"
	mapping                   = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
)

var (
	singleton *StoredQuery
	once      sync.Once
)

// StoredQuery plugin deals with managing query templates.
type StoredQuery struct {
	es storedQueryService
}

// Instance returns the singleton instance of the plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *StoredQuery {
	once.Do(func() { singleton = &StoredQuery{} })
	return singleton
}

// Name returns the name of the plugin: [storedquery]
func (s *StoredQuery) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (s *StoredQuery) InitFunc() error {
	indexPrefix := os.Getenv(envStoredQueryEsIndex)
	if indexPrefix == "" {
		indexPrefix = defaultStoredQueryEsIndex
	}
	indexPrefix = util.MetaIndexName(indexPrefix)

	// initialize the dao
	var err error
	s.es, err = initPlugin(indexPrefix, mapping)
	if err != nil {
		log.Errorln(logTag, ":", err.Error())
		return err
	}
	// sync stored queries
	esStoredQueries, err := s.es.getStoredQueries(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", "error while retrieving the stored queries:", err)
		return nil
	}
	err2 := SetStoredQueriesToCache(esStoredQueries)
	if err2 != nil {
		log.Errorln(logTag, ":", err2.Error())
		return nil
	}

	// Set plugin cache sync script
	script := CacheSyncScript{
		index: indexPrefix,
	}
	util.AddSyncScript(script)

	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (r *StoredQuery) Routes() []plugins.Route {
	return r.routes()
}

func (r *StoredQuery) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

func (r *StoredQuery) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		r.intercept,
	}
}

// Expose plugin specific routes
func (r *StoredQuery) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
