package permissions

import (
	"os"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
)

const (
	logTag                    = "[permissions]"
	defaultPermissionsEsIndex = ".permissions"
	typeName                  = "_doc"
	envEsURL                  = "ES_CLUSTER_URL"
	envPermissionEsIndex      = "PERMISSIONS_ES_INDEX"
	settings                  = `{ "settings" : { %s "index.number_of_shards" : 1, "index.number_of_replicas" : %d } }`
)

var (
	singleton *permissions
	once      sync.Once
)

type permissions struct {
	es permissionService
}

// Use only this function to fetch the instance of permission from within
// this package to avoid creating stateless duplicates of the plugin.
func Instance() *permissions {
	once.Do(func() { singleton = &permissions{} })
	return singleton
}

func (p *permissions) Name() string {
	return logTag
}

func (p *permissions) InitFunc() error {
	log.Println(logTag, ": initializing plugin")

	indexName := os.Getenv(envPermissionEsIndex)
	if indexName == "" {
		indexName = defaultPermissionsEsIndex
	}

	// initialize the dao
	var err error
	p.es, err = initPlugin(indexName, settings)
	if err != nil {
		return err
	}

	// Set plugin cache sync script
	s := CacheSyncScript{
		index: indexName,
	}
	util.AddSyncScript(s)

	return nil
}

func (p *permissions) Routes() []plugins.Route {
	return p.routes()
}

// Default empty middleware array function
func (p *permissions) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (p *permissions) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
