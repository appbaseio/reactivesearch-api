package uibuilder

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
	logTag   = "[uibuilder]"
	typeName = "_doc"
	// Old index which contains the older records that
	// would be migrated to new index (.uibuilder_preferences)
	defaultEcommPreferencesIndex = ".uibuilder-preferences"
	envEcommPreferencesIndex     = "ECOMM_PREFERENCES_ES_INDEX"
	// New index to store the preferences
	defaultUIBuilderPreferencesIndex = ".uibuilder_preferences"
	envUIBuilderPreferencesIndex     = "UIBUILDER_PREFERENCES_ES_INDEX"
	mapping                          = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
	envSearchBoxIndex                = "SEARCHBOX_ES_INDEX"
	defaultSearchBoxIndex            = ".searchbox"
	indexConfigZinc                  = `
	{
		"name": %s,
		"mappings":{
			"properties":{
				"key":{
					"type":"text",
					"fields":{
						"autosuggest":{
							"type":"text",
							"analyzer":"autosuggest_analyzer"
						},
						"keyword":{
							"type":"keyword",
							"ignore_above":256
						}
					}
				},
				"count":{
					"type":"integer"
				}
			}
		}
	 }
	`
	indexSettingsZinc = `
	{
		"settings":{
			%s
		   "index.number_of_shards": 1,
		   "index.number_of_replicas": %d,
		   "analysis":{
			  "analyzer":{
					"autosuggest_analyzer":{
						"filter":[
							"lowercase",
							"asciifolding",
							"autosuggest_filter"
						],
						"tokenizer":"standard",
						"type":"custom"
					}
			  },
			  "filter":{
					"autosuggest_filter":{
						"max_gram":"20",
						"min_gram":"1",
						"token_chars":[
							"letter",
							"digit",
							"punctuation",
							"symbol"
						],
						"type":"edge_ngram"
					}
			  }
		   }
		}
	 }
	`
)

var (
	singleton *UIBuilder
	once      sync.Once
)

type UIBuilder struct {
	es                        uiBuilderService
	featuredSuggestionsConfig FeaturedSuggestionsConfig
	esFeaturedSuggestions     searchboxService
}

type elasticsearch struct {
	indexName string
}

// Instance returns the singleton instance of Logs plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Logs in order to avoid stateless instances of the plugin.
func Instance() *UIBuilder {
	once.Do(func() { singleton = &UIBuilder{} })
	return singleton
}

// Name returns the name of the plugin: "[logs]"
func (e *UIBuilder) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (e *UIBuilder) InitFunc() error {
	preferencesIndex := os.Getenv(envUIBuilderPreferencesIndex)
	if preferencesIndex == "" {
		preferencesIndex = defaultUIBuilderPreferencesIndex
	}
	searchboxIndex := os.Getenv(envSearchBoxIndex)
	if searchboxIndex == "" {
		searchboxIndex = defaultSearchBoxIndex
	}
	var err error
	e.esFeaturedSuggestions, _, err = createSearchBoxIndex(searchboxIndex, mapping)
	if err != nil {
		return err
	}
	// create searchbox index in zinc
	_, _, err2 := createSuggestionsIndexZinc(searchboxIndex)
	if err2 != nil {
		log.Errorln(logTag, ": error creating suggestions index in zinc, ", searchboxIndex, ":", err2)
		return err
	}
	e.featuredSuggestionsConfig = FeaturedSuggestionsConfig{
		zincIndex: searchboxIndex,
	}

	// initialize the dao
	e.es, err = initPlugin(preferencesIndex, mapping)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return err
	}
	oldPreferenceIndex := os.Getenv(envEcommPreferencesIndex)
	if oldPreferenceIndex == "" {
		oldPreferenceIndex = defaultEcommPreferencesIndex
	}
	// Add suggestions preferences migration script
	m := UIBuilderPreferencesMigration{
		newIndex: preferencesIndex,
		oldIndex: oldPreferenceIndex,
	}
	util.AddMigrationScript(m)

	// sync searchbox preferences cache
	searchboxPreferencesResponse, err := util.GetClient7().
		Search(searchboxIndex).
		Size(10000).
		Do(context.Background())
	if err != nil {
		return err
	}
	if searchboxPreferencesResponse != nil {
		err := e.featuredSuggestionsConfig.setFeaturedSuggestionsFromESResponse(searchboxPreferencesResponse, searchboxIndex)
		if err != nil {
			return err
		}
	}
	// Set featured suggestions cache sync script
	f := FeaturedSuggestionsCacheSyncScript{
		index:                     searchboxIndex,
		featuredSuggestionsConfig: e.featuredSuggestionsConfig,
	}
	util.AddSyncScript(f)
	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (e *UIBuilder) Routes() []plugins.Route {
	return e.routes()
}

// ESMiddleware is a default empty middleware function
func (p *UIBuilder) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// RSMiddleware is a default empty middleware function
func (p *UIBuilder) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Expose plugin specific routes
func (p *UIBuilder) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
