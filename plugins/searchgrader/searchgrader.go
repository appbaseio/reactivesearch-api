package searchgrader

import (
	"os"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
)

const (
	logTag                     = "[searchgrader]"
	defaultSearchgraderEsIndex = ".searchgrader"
	envSearchgraderEsIndex     = "SEARCH_GRADER_ES_INDEX"
	typeName                   = "_doc"
	mapping                    = `{ "settings": { "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
)

var (
	instance *SearchGrader
	once     sync.Once
)

// SearchGrader plugin exposes the routes to record and retrieve search grades.
type SearchGrader struct {
	es searchGraderService
}

// Instance returns the singleton instace of SearchGrader plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance searchgrader in order to avoid stateless instances of the plugin.
func Instance() *SearchGrader {
	once.Do(func() { instance = &SearchGrader{} })
	return instance
}

// Name is a part of Plugin interface that returns the name of the plugin: '[searchgrader]'.
func (s *SearchGrader) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (s *SearchGrader) InitFunc() error {
	// fetch the required env vars
	searchgraderIndex := os.Getenv(envSearchgraderEsIndex)
	if searchgraderIndex == "" {
		searchgraderIndex = defaultSearchgraderEsIndex
	}
	searchgraderIndex = util.MetaIndexName(searchgraderIndex)
	// initialize the dao
	var err error
	s.es, err = initPlugin(searchgraderIndex, mapping)
	if err != nil {
		return err
	}
	return nil
}

// Routes returns the searchgrader routes that the plugin serves.
func (s *SearchGrader) Routes() []plugins.Route {
	return s.routes()
}
func (s *SearchGrader) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

func (s *SearchGrader) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

// Expose plugin specific routes
func (s *SearchGrader) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
