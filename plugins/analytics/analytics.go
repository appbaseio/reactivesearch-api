package analytics

import (
	"os"
	"sync"
	"net/http"

	"github.com/appbaseio-confidential/arc/plugins"
	"github.com/appbaseio-confidential/arc/errors"
	"github.com/appbaseio-confidential/arc/middleware"
)

const (
	logTag                  = "[analytics]"
	defaultAnalyticsEsIndex = ".analytics"
	envAnalyticsEsIndex     = "ANALYTICS_ES_INDEX"
	defaultLogsEsIndex      = ".logs"
	envLogsEsIndex          = "LOGS_ES_INDEX"
	envEsURL                = "ES_CLUSTER_URL"
	mapping                 = `{ "settings": { "number_of_shards": %d, "number_of_replicas": %d } }`
)

var (
	instance *Analytics
	once     sync.Once
)

// Analytics plugin records and serves basic index or cluster level analytics.
type Analytics struct {
	es analyticsService
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
func (a *Analytics) InitFunc(_ [] middleware.Middleware) error {
	// fetch the required env vars
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	analyticsIndex := os.Getenv(envAnalyticsEsIndex)
	if analyticsIndex == "" {
		analyticsIndex = defaultAnalyticsEsIndex
	}
	logsIndex := os.Getenv(envLogsEsIndex)
	if logsIndex == "" {
		logsIndex = defaultLogsEsIndex
	}

	// initialize the dao
	var err error
	a.es, err = newClient(url, analyticsIndex, logsIndex, mapping)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns the analytics routes that the plugin serves.
func (a *Analytics) Routes() []plugins.Route {
	return a.routes()
}

func (a *Analytics) ESMiddleware() []middleware.Middleware {
	return [] middleware.Middleware {
		func(h http.HandlerFunc) http.HandlerFunc {
			return a.recorder(h)
		},
	}
}
