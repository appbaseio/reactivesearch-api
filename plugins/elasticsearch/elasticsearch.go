package elasticsearch

import (
	"sync"

	"github.com/appbaseio-confidential/arc/plugins"
	"github.com/appbaseio-confidential/arc/middleware"
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

func (es *elasticsearch) InitFunc(mw [] middleware.Middleware) error {
	return es.preprocess(mw)
}

func (es *elasticsearch) Routes() []plugins.Route {
	return es.routes()
}

// Default empty middleware array function
func (es *elasticsearch) ESMiddleware() [] middleware.Middleware {
	return make([] middleware.Middleware, 0)
}
