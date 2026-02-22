package searchrelevancy

import (
	"context"
	"os"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"
)

const (
	logTag                        = "[searchrelevancy]"
	defaultSearchRelevancyEsIndex = ".searchrelevancy"
	envSearchRelevancyEsIndex     = "SEARCH_RELEVANCY_ES_INDEX"
	mapping                       = `{"properties": {"search": {"properties": {"fieldWeights": {"type": "float"}, "fuzziness": {"type": "keyword"}}}}}`
	indexSettingMapping           = `{ "mappings": %s, "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
)

var (
	instance *SearchRelevancy
	once     sync.Once
)

// SearchRelevancy plugin saves default settings for search, aggregations, result, language.
type SearchRelevancy struct {
	es       searchRelevancyService
	validate *validator.Validate
}

// Instance returns the singleton instace of SearchRelevancy plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance searchRelevancySettings in order to avoid stateless instances of the plugin.
func Instance() *SearchRelevancy {
	once.Do(func() { instance = &SearchRelevancy{} })
	return instance
}

// Name is a part of Plugin interface that returns the name of the plugin: '[searchsetttings]'.
func (a *SearchRelevancy) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (a *SearchRelevancy) InitFunc() error {
	// fetch the required env vars
	searchRelevancyIndex := os.Getenv(envSearchRelevancyEsIndex)
	if searchRelevancyIndex == "" {
		searchRelevancyIndex = defaultSearchRelevancyEsIndex
	}

	// initialize the dao
	var err error
	a.es, err = initPlugin(searchRelevancyIndex)
	if err != nil {
		return err
	}

	relevancySettings, err := a.es.getSearchRelevancySettings(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", "error while retrieving the searchRelevancySettings:", err)
		return nil
	}
	SetSearchRelevancySettingsCache(relevancySettings)

	// initialize validator
	a.validate = validator.New()

	// Set plugin cache sync script
	s := CacheSyncScript{
		index: searchRelevancyIndex,
	}
	util.AddSyncScript(s)

	return nil
}

// Routes returns the searchrelevancy routes that the plugin serves.
func (a *SearchRelevancy) Routes() []plugins.Route {
	return a.routes()
}

// ESMiddleware ES middlewares
func (a *SearchRelevancy) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

// RSMiddleware RS middlewares
func (a *SearchRelevancy) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		a.intercept,
	}
}

// Expose plugin specific routes
func (a *SearchRelevancy) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
