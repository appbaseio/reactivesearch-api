package es

import (
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
)

const logTag = "[es]"

var (
	instance *es
	once     sync.Once
)

type es struct {
	specs []api
}

func init() {
	arc.RegisterPlugin(Instance())
}

func Instance() *es {
	once.Do(func() {
		instance = &es{}
	})
	return instance
}

func (es *es) Name() string {
	return logTag
}

func (es *es) InitFunc() error {
	return nil
}

func (es *es) Routes() []plugin.Route {
	return es.routes()
}
