package zinc

import (
	"sync"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

const (
	logTag = "[zinc]"
)

// Zinc plugin deals with managing zinc related stuff.
var (
	singleton *Zinc
	once      sync.Once
)

// Zinc plugin deals with managing zinc related details.
type Zinc struct {
}

// Instance returns the singleton instance of the plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *Zinc {
	once.Do(func() { singleton = &Zinc{} })
	return singleton
}

// Name returns the name of the plugin: [pipelines]
func (r *Zinc) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (p *Zinc) InitFunc() error {
	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (p *Zinc) Routes() []plugins.Route {
	return []plugins.Route{}
}

func (p *Zinc) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

func (p *Zinc) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

// Expose plugin specific routes
func (p *Zinc) AlternateRoutes() []plugins.Route {
	return []plugins.Route{}
}
