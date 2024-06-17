package synonyms

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/reactivesearch/plugins"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/go-playground/validator/v10"
)

const (
	logTag                 = "[synonyms]"
	defaultSynonymsEsIndex = ".synonyms"
	synonymsEsIndex        = "SYNONYMS_ES_INDEX"
	mapping                = `{ "settings": { "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
	typeName               = "_doc"
)

var (
	instance *Synonyms
	once     sync.Once
)

// Synonyms plugin saves default settings for search, aggregations, result, language.
type Synonyms struct {
	es       synonymsService
	validate *validator.Validate
}

// Instance returns the singleton instace of Synonyms plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance synonyms in order to avoid stateless instances of the plugin.
func Instance() *Synonyms {
	once.Do(func() { instance = &Synonyms{} })
	return instance
}

// Name is a part of Plugin interface that returns the name of the plugin: '[synonyms]'.
func (s *Synonyms) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (s *Synonyms) InitFunc() error {
	// fetch the required env vars
	synonymsIndex := os.Getenv(synonymsEsIndex)
	if synonymsIndex == "" {
		synonymsIndex = defaultSynonymsEsIndex
	}

	// initialize the dao
	var err error
	s.es, err = initPlugin(synonymsIndex, mapping)
	if err != nil {
		return err
	}

	// initialize validator
	s.validate = validator.New()
	return nil
}

// Routes returns the synonyms routes that the plugin serves.
func (s *Synonyms) Routes() []plugins.Route {
	return s.routes()
}

// ESMiddleware ES middlewares
func (s *Synonyms) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

// RSMiddleware RS middlewares
func (s *Synonyms) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}

}

// Expose plugin specific routes
func (s *Synonyms) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
