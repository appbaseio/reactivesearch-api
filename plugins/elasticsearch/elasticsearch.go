package elasticsearch

import (
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/route"
)

const logTag = "[elasticsearch]"

var (
	instance *Elasticsearch
	once     sync.Once
)

type Elasticsearch struct {
	specs []api
}

func init() {
	arc.RegisterPlugin(Instance())
}

func Instance() *Elasticsearch {
	once.Do(func() { instance = &Elasticsearch{} })
	return instance
}

func (es *Elasticsearch) Name() string {
	return logTag
}

func (es *Elasticsearch) InitFunc() error {
	return es.preprocess()
}

func (es *Elasticsearch) Routes() []route.Route {
	return es.routes()
}
