package analytics

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/robfig/cron"
)

const (
	logTag                        = "[analytics]"
	separator                     = "/0"
	defaultAnalyticsEsIndex       = ".analytics"
	envAnalyticsEsIndex           = "ANALYTICS_ES_INDEX"
	defaultsSavedSearchesEsIndex  = ".saved_searches"
	envSavedSearchesEsIndex       = "SAVED_SEARCHES_ES_INDEX"
	defaultsFavoritesEsIndex      = ".favorites"
	envSavedFavoritesEsIndex      = "FAVORITES_ES_INDEX"
	defaultLogsEsIndex            = ".logs"
	envLogsEsIndex                = "LOGS_ES_INDEX"
	defaultUsersEsIndex           = ".users"
	envUsersEsIndex               = "USERS_ES_INDEX"
	defaultUserSessionIndex       = ".user_sessions"
	envUserSessionIndex           = "USER_SESSION_ES_INDEX"
	defaultAnalyticsInsightsIndex = ".actionableinsights"
	envAnalyticsInsightsIndex     = "ANALYTICS_INSIGHTS_ES_INDEX"
	defaultPreferencesIndex       = ".analytics_preferences"
	envPreferencesIndex           = "ANALYTICS_PREFERENCES_INDEX"
	envRecentSearchesEsIndex      = "RECENT_SEARCHES_ES_INDEX"
	defaultRecentSearchesEsIndex  = ".documents"
	indexSettingsRecentSearches   = `{
		"settings":{
			%s
		   "index.number_of_shards": 2,
		   "index.number_of_replicas": %d
		},
		"mappings": %s
	}`
	defaultUserSessionDuration = 30 // in minutes
	userSessionMapping         = `{ "mappings": %s, "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
	mapping                    = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
	analyticsMapping           = `{"aliases":{"%s":{"is_write_index":true}}, "mappings": %s, "settings":{ %s "index.number_of_shards":1,"index.number_of_replicas":%d}}`
	preferencesMapping         = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d } }`
	CustomEventsPrefix         = "c_"
	typeName                   = "_doc"
	preferencesDocId           = "analytics_prefs"
	rolloverConfig             = `{
		"max_age":  "%s",
		"max_docs": %d,
		"max_size": "%s"
	}`
	arcUUID     = "ARC_ID"
	clusterUUID = "CLUSTER_ID"
)

var (
	instance *Analytics
	once     sync.Once
)

// Analytics plugin records and serves basic index or cluster level analytics.
type Analytics struct {
	es               analyticsService
	session          *TTLMap
	userSession      *ActiveUserSessionTTLMap
	timestampSession *TTLMapTimestamp
	recentDocuments  recentDocumentsService
}

// Instance returns the singleton instace of Analytics plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance analytics in order to avoid stateless instances of the plugin.
func Instance() *Analytics {
	once.Do(func() { instance = &Analytics{} })
	return instance
}

// Name is a part of Plugin interface that returns the name of the plugin: '[analytics]'.
func (a *Analytics) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (a *Analytics) InitFunc() error {
	// fetch the required env vars
	analyticsIndex := os.Getenv(envAnalyticsEsIndex)
	if analyticsIndex == "" {
		analyticsIndex = defaultAnalyticsEsIndex
	}
	logsIndex := os.Getenv(envLogsEsIndex)
	if logsIndex == "" {
		logsIndex = defaultLogsEsIndex
	}

	userSessionIndex := os.Getenv(envUserSessionIndex)
	if userSessionIndex == "" {
		userSessionIndex = defaultUserSessionIndex
	}

	analyticsInsightsIndex := os.Getenv(envAnalyticsInsightsIndex)
	if analyticsInsightsIndex == "" {
		analyticsInsightsIndex = defaultAnalyticsInsightsIndex
	}

	usersIndex := os.Getenv(envUsersEsIndex)
	if usersIndex == "" {
		usersIndex = defaultUsersEsIndex
	}
	savedSearchedIndex := os.Getenv(envSavedFavoritesEsIndex)
	if savedSearchedIndex == "" {
		savedSearchedIndex = defaultsSavedSearchesEsIndex
	}
	favoritesIndex := os.Getenv(envSavedFavoritesEsIndex)
	if favoritesIndex == "" {
		favoritesIndex = defaultsFavoritesEsIndex
	}

	preferencesIndex := os.Getenv(envPreferencesIndex)
	if preferencesIndex == "" {
		preferencesIndex = defaultPreferencesIndex
	}

	recentDocumentsIndex := os.Getenv(envRecentSearchesEsIndex)
	if recentDocumentsIndex == "" {
		recentDocumentsIndex = defaultRecentSearchesEsIndex
	}

	// initialize the dao
	var err error
	a.es, err = initPlugin(analyticsIndex, logsIndex, usersIndex, userSessionIndex, analyticsInsightsIndex, savedSearchedIndex, favoritesIndex, mapping, analyticsMapping,
		preferencesIndex, preferencesMapping)
	if err != nil {
		return err
	}

	// Create the recent documents index
	a.recentDocuments, err = createRecentSearchesIndex(recentDocumentsIndex, indexSettingsRecentSearches)
	if err != nil {
		return err
	}

	// Create a session to record search ids with a 30s max TTL
	a.session = InitSession(10000, 30)
	// Create a session to record user ids against latest request timestamp with a 30s max TTL
	a.timestampSession = InitTimestampSession(10000, 30)
	/**
	Create a session to record active user sessions.
	Note: At a particular time maximum active users limit can be 1,000,000.
	*/
	activeUserSessions, err := a.es.getActiveUserSessions(context.Background())
	if err != nil {
		return err
	}
	// If active sessions are present then initialize the session map with active sessions
	if len(activeUserSessions) != 0 {
		var m = make(map[string]*ActiveUserSessionItem)
		for _, v := range activeUserSessions {
			activeSessionItem := ActiveUserSessionItem{
				value: ActiveUserSession{
					StartTime: *v.UserSession.StartTime,
					Bounce:    *v.UserSession.Bounce,
					ID:        v.ID,
					TimeStamp: v.UserSession.TimeStamp,
				},
				lastAccess: *v.UserSession.LastInteractionTime,
			}
			if v.UserSession.UserID != nil {
				m[*v.UserSession.UserID] = &activeSessionItem
			}
		}
		initialValue := ActiveUserSessionTTLMap{m: m}
		a.userSession = InitUserSession(60000, defaultUserSessionDuration*60, &initialValue)
	} else {
		a.userSession = InitUserSession(60000, defaultUserSessionDuration*60, nil)
	}

	// Fetch the analytics preferences and save them in the cache
	analyticsPrefs, fetchErr := a.es.getPreferences(context.Background())
	if fetchErr != nil {
		return fmt.Errorf("Error while fetching analytics preferences: %s", fetchErr.Error())
	}
	SetPreferencesInCache(analyticsPrefs)

	// init cron job
	cronjob := cron.New()
	cronjob.AddFunc("@midnight", func() { a.es.rolloverIndexJob(analyticsIndex) })
	cronjob.Start()

	// init a monthly cron job to send analytics report to the admin users
	analyticsCronJob := cron.New()
	// Run once a month, midnight, first of month
	analyticsCronJob.AddFunc("@monthly", a.es.reportAnalyticsToUsers)
	analyticsCronJob.Start()

	// Add analytics mapping changes migration script
	m := MappingsMigration{
		NewMapping: getAnalyticsMappings(),
		es:         a.es.(*elasticsearch),
	}
	util.AddMigrationScript(m)
	// Add user session mapping changes to migration script
	util.AddMigrationScript(UserSessionMappingsMigration{
		NewMapping: getUserSessionMappings(),
		es:         a.es.(*elasticsearch),
		indexName:  userSessionIndex,
	})
	clusterBilling := util.ClusterBilling

	if clusterBilling == "true" {
		cronjob.AddFunc("@midnight", a.es.deleteOldMetricBeatIndices)
	}
	return nil
}

// Routes returns the analytics routes that the plugin serves.
func (a *Analytics) Routes() []plugins.Route {
	return a.routes()
}

func (a *Analytics) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		func(h http.HandlerFunc) http.HandlerFunc {
			return a.recorder(h)
		},
	}
}

func (a *Analytics) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		a.initContext,
		a.recorder,
	}
}

// Expose plugin specific routes
func (a *Analytics) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
