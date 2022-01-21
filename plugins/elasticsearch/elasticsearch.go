package elasticsearch

import (
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
)

const logTag = "[elasticsearch]"

var (
	singleton *elasticsearch
	once      sync.Once
)

type elasticsearch struct {
	specs []api
}

func Instance() *elasticsearch {
	once.Do(func() { singleton = &elasticsearch{} })
	return singleton
}

func (es *elasticsearch) Name() string {
	return logTag
}

func (es *elasticsearch) InitFunc(mw []middleware.Middleware) error {
	return es.preprocess(mw)
}

func (es *elasticsearch) Routes() []plugins.Route {
	return es.routes()
}

// Alternate routes
func (a *elasticsearch) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}

// Default empty middleware array function
func (es *elasticsearch) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
