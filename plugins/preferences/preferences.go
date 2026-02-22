package preferences

import (
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
)

const (
	logTag   = "[preferences]"
	typeName = "_doc"
)

var (
	singleton *preferences
	once      sync.Once
)

type preferences struct {
}

// Use only this function to fetch the instance of user from within
// this package to avoid creating stateless duplicates of the plugin.
func Instance() *preferences {
	once.Do(func() { singleton = &preferences{} })
	return singleton
}

func (rx *preferences) Name() string {
	return logTag
}

func (rx *preferences) InitFunc() error {
	return nil
}

func (rx *preferences) Routes() []plugins.Route {
	return rx.routes()
}

// Default empty middleware array function
func (rx *preferences) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (rx *preferences) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Expose plugin specific routes
func (rx *preferences) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
