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

type ES struct {
	specs []api
}

func init() {
	arc.RegisterPlugin(&ES{})
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
