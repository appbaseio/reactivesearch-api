package sync

import (
	"context"
	"os"
	"sync"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

const (
	logTag                      = "[sync]"
	syncPreferenceDocID         = "sync_preferences"
	defaultSyncPreferencesIndex = ".sync_preferences"
	envSyncPreferencesEsIndex   = "SYNC_PREFERENCES_ES_INDEX"
	mapping                     = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
)

var (
	singleton *Sync
	once      sync.Once
)

type Sync struct {
	es syncService
}

// Instance returns the singleton instance of Sync plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Sync in order to avoid stateless instances of the plugin.
func Instance() *Sync {
	once.Do(func() { singleton = &Sync{} })
	return singleton
}

// Name returns the name of the plugin
func (p *Sync) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (p *Sync) InitFunc() error {

	// Create suggestions preferences index if not exists
	indexName := os.Getenv(envSyncPreferencesEsIndex)
	if indexName == "" {
		indexName = defaultSyncPreferencesIndex
	}

	// initialize the dao
	var err error
	p.es, err = initPlugin(indexName, mapping)
	if err != nil {
		return err
	}

	// sync preferences
	preferences, err := p.es.getSyncPreferences(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
	} else {
		setPreferences(preferences)
	}
	// Set plugin cache sync script
	s := CacheSyncScript{
		index: indexName,
	}
	util.AddSyncScript(s)

	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (p *Sync) Routes() []plugins.Route {
	return p.routes()
}

// ESMiddleware is a default empty middleware function
func (p *Sync) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// RSMiddleware is a default empty middleware function
func (p *Sync) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Expose plugin specific routes
func (p *Sync) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
