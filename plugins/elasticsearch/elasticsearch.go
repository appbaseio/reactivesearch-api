package elasticsearch

import (
	"sync"

	"github.com/appbaseio-confidential/arc/arc/route"
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

func (es *elasticsearch) InitFunc() error {
	return es.preprocess()
}

func (es *elasticsearch) Routes() []route.Route {
	return es.routes()
}
