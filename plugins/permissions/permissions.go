package permissions

import (
	"log"
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
)

const (
	logTag               = "[permissions]"
	envEsURL             = "ES_CLUSTER_URL"
	envPermissionEsIndex = "PERMISSIONS_ES_INDEX"
)

var (
	singleton *permissions
	once      sync.Once
)

type permissions struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(instance())
}

// Use only this function to fetch the instance of permission from within
// this package to avoid creating stateless duplicates of the plugin.
func instance() *permissions {
	once.Do(func() { singleton = &permissions{} })
	return singleton
}

func (p *permissions) Name() string {
	return logTag
}

func (p *permissions) InitFunc() error {
	log.Printf("%s: initializing plugin\n", logTag)

	// fetch vars from env
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envPermissionEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envPermissionEsIndex)
	}
	mapping := permission.IndexMapping

	// initialize the dao
	var err error
	p.es, err = newClient(url, indexName, mapping)
	if err != nil {
		return err
	}

	return nil
}

func (p *permissions) Routes() []route.Route {
	return p.routes()
}
