package nodes

import (
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	log "github.com/sirupsen/logrus"
)

const (
	logTag            = "[nodes]"
	defaultNodesIndex = ".nodes"
	typeName          = "_doc"
	settings          = `{ "settings" : { %s "index.number_of_shards" : 1, "index.number_of_replicas" : %d } }`
)

var (
	singleton *nodes
	once      sync.Once
)

type nodes struct {
	es nodeService
}

// Use only this function to fetch the instance of nodes from within
// this package to avoid creating stateless duplicates of the plugin.
func Instance() *nodes {
	once.Do(func() { singleton = &nodes{} })
	return singleton
}

func (n *nodes) Name() string {
	return logTag
}

func (n *nodes) InitFunc() error {
	log.Println(logTag, ": initializing plugin")

	indexName := defaultNodesIndex

	// initialize the dao
	var err error
	n.es, err = initPlugin(indexName, settings)
	if err != nil {
		return err
	}

	return nil
}

func (n *nodes) Routes() []plugins.Route {
	return n.routes()
}

// Default empty middleware array function
func (n *nodes) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (n *nodes) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

func (n *nodes) Enabled() bool {
	return true
}
