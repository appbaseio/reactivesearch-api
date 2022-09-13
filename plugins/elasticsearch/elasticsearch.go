package elasticsearch

import (
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
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

// Default empty middleware array function
func (es *elasticsearch) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Plugin is enabled only when external ES is used
func (es *elasticsearch) Enabled() bool {
	return util.IsSLSDisabled()
}
