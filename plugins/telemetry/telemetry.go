package telemetry

import (
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
)

const (
	logTag            = "[telemetry]"
	eventType         = "telemetry" // New relic event name
	frontEndHeader    = "X-Search-Client"
	telemetryHeader   = "X-Enable-Telemetry"
	defaultServerMode = "OSS"
)

var blacklistRoutes = []string{"/"}

var (
	singleton *Telemetry
	once      sync.Once
)

// Telemetry plugin records the API usage.
type Telemetry struct{}

// Instance returns the singleton instance of Telemetry plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Logs in order to avoid stateless instances of the plugin.
func Instance() *Telemetry {
	once.Do(func() { singleton = &Telemetry{} })
	return singleton
}

// Name returns the name of the plugin: "[telemetry]"
func (t *Telemetry) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (t *Telemetry) InitFunc() error {
	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (t *Telemetry) Routes() []plugins.Route {
	return []plugins.Route{}
}

// Default empty middleware array function
func (t *Telemetry) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (t *Telemetry) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
