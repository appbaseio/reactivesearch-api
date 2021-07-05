package querytranslate

import (
	"sync"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/plugins"
)

const (
	logTag   = "[querytranslate]"
	typeName = "_doc"
)

var (
	singleton *QueryTranslate
	once      sync.Once
)

// QueryTranslate plugin deals with managing query translation.
type QueryTranslate struct{}

// Instance returns the singleton instance of the plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *QueryTranslate {
	once.Do(func() { singleton = &QueryTranslate{} })
	return singleton
}

// Name returns the name of the plugin: [querytranslate]
func (r *QueryTranslate) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (r *QueryTranslate) InitFunc(mw []middleware.Middleware) error {
	return r.preprocess(mw)
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (r *QueryTranslate) Routes() []plugins.Route {
	return r.routes()
}

func (r *QueryTranslate) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
