package uibuilder

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
	// Separate index for denormalized featured suggestions (for efficient search)
	envFeaturedSuggestionsIndex     = "FEATURED_SUGGESTIONS_ES_INDEX"
	defaultFeaturedSuggestionsIndex = ".featured_suggestions"
	featuredSuggestionsMapping      = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d }, "mappings": { "properties": { "label": { "type": "text" }, "value": { "type": "text" }, "description": { "type": "text" }, "action": { "type": "keyword" }, "subAction": { "type": "keyword" }, "searchboxId": { "type": "keyword" }, "sectionId": { "type": "keyword" }, "sectionLabel": { "type": "text" }, "icon": { "type": "keyword" }, "iconURL": { "type": "keyword" }, "order": { "type": "integer" } } } }`
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
	if !util.ShouldCreateMetaIndex(util.MetaIndexUIBuilderPreferences) &&
		!util.ShouldCreateMetaIndex(util.MetaIndexSearchBox) &&
		!util.ShouldCreateMetaIndex(util.MetaIndexFeaturedSuggestions) {
		log.Infoln(logTag, ": skipping ES index creation (setup profile:", util.GetSetupProfile(), ")")
		return nil
	}

	preferencesIndex := os.Getenv(envUIBuilderPreferencesIndex)
	if preferencesIndex == "" {
		preferencesIndex = defaultUIBuilderPreferencesIndex
	}
	preferencesIndex = util.MetaIndexName(preferencesIndex)
	searchboxIndex := os.Getenv(envSearchBoxIndex)
	if searchboxIndex == "" {
		searchboxIndex = defaultSearchBoxIndex
	}
	searchboxIndex = util.MetaIndexName(searchboxIndex)
	var err error
	if util.ShouldCreateMetaIndex(util.MetaIndexSearchBox) {
		e.esFeaturedSuggestions, _, err = createSearchBoxIndex(searchboxIndex, mapping)
		if err != nil {
			return err
		}
	}
	// Create separate index for denormalized featured suggestions
	featuredSuggestionsIndex := os.Getenv(envFeaturedSuggestionsIndex)
	if featuredSuggestionsIndex == "" {
		featuredSuggestionsIndex = defaultFeaturedSuggestionsIndex
	}
	featuredSuggestionsIndex = util.MetaIndexName(featuredSuggestionsIndex)
	var featuredSuggestionsIndexExists bool
	if util.ShouldCreateMetaIndex(util.MetaIndexFeaturedSuggestions) {
		_, featuredSuggestionsIndexExists, err = createSearchBoxIndex(featuredSuggestionsIndex, featuredSuggestionsMapping)
		if err != nil {
			return err
		}
		e.featuredSuggestionsConfig = FeaturedSuggestionsConfig{
			esIndex: featuredSuggestionsIndex,
		}
	}

	// initialize the dao
	if util.ShouldCreateMetaIndex(util.MetaIndexUIBuilderPreferences) {
		e.es, err = initPlugin(preferencesIndex, mapping)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
	}
	oldPreferenceIndex := os.Getenv(envEcommPreferencesIndex)
	if oldPreferenceIndex == "" {
		oldPreferenceIndex = defaultEcommPreferencesIndex
	}
	if util.ShouldCreateMetaIndex(util.MetaIndexUIBuilderPreferences) {
		util.AddMigrationScript(UIBuilderPreferencesMigration{
			newIndex: preferencesIndex,
			oldIndex: oldPreferenceIndex,
		})
	}

	if util.ShouldCreateMetaIndex(util.MetaIndexSearchBox) {
		searchboxPreferencesResponse, searchErr := util.GetClient7().
			Search(searchboxIndex).
			Size(10000).
			Do(context.Background())
		if searchErr != nil {
			return searchErr
		}
		if searchboxPreferencesResponse != nil && util.ShouldCreateMetaIndex(util.MetaIndexFeaturedSuggestions) {
			syncToES := !featuredSuggestionsIndexExists
			err := e.featuredSuggestionsConfig.setFeaturedSuggestionsFromESResponse(searchboxPreferencesResponse, searchboxIndex, syncToES)
			if err != nil {
				return err
			}
		}
		util.AddSyncScript(FeaturedSuggestionsCacheSyncScript{
			index:                     searchboxIndex,
			featuredSuggestionsConfig: e.featuredSuggestionsConfig,
		})
	}
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
