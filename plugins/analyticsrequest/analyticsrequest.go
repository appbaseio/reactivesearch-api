package analyticsrequest

import (
	"sync"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

const (
	logTag = "[analyticsrequest]"
)

var (
	singleton *AnalyticsRequest
	once      sync.Once
)

type AnalyticsRequest struct{}

// Name returns the name of the plugin: "[analyticsrequest]"
func (f *AnalyticsRequest) Name() string {
	return logTag
}

// Instance returns the singleton instace of AnalyticsRequest plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance AnalyticsRequest in order to avoid stateless instances of the plugin.
func Instance() *AnalyticsRequest {
	once.Do(func() { singleton = &AnalyticsRequest{} })
	return singleton
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (c *AnalyticsRequest) InitFunc() error {
	return nil
}

func (c *AnalyticsRequest) Routes() []plugins.Route {
	return []plugins.Route{}
}

func (c *AnalyticsRequest) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

func (c *AnalyticsRequest) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		trackAnalyticsRequestPlugin,
	}
}

// Expose plugin specific routes
func (a *AnalyticsRequest) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
