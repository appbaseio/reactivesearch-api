package reindexer

import (
	"sync"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/plugins"
)

const (
	logTag   = "[reindexer]"
	envEsURL = "ES_CLUSTER_URL"
	typeName = "_doc"
)

var (
	singleton *reindexer
	once      sync.Once
)

type reindexer struct {
}

// Use only this function to fetch the instance of user from within
// this package to avoid creating stateless duplicates of the plugin.
func Instance() *reindexer {
	once.Do(func() { singleton = &reindexer{} })
	return singleton
}

func (rx *reindexer) Name() string {
	return logTag
}

func (rx *reindexer) InitFunc() error {
	InitIndexAliasCache()
	InitAliasIndexCache()
	return nil
}

func (rx *reindexer) Routes() []plugins.Route {
	return rx.routes()
}

// Default empty middleware array function
func (rx *reindexer) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (rx *reindexer) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
