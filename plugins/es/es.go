package es

import (
	"log"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
)

const (
	pluginName = "es"
	logTag     = "[es]"
)

type ES struct{}

func init() {
	arc.RegisterPlugin(New())
}

func New() *ES {
	return &ES{}
}

func (es *ES) Name() string {
	return pluginName
}

func (es *ES) InitFunc() error {
	log.Printf("%s: initializing plugin: %s", logTag, pluginName)
	return nil
}

func (es *ES) Routes() []plugin.Route {
	return es.routes()
}
