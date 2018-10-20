package permissions

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
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

func instance() *permissions {
	once.Do(func() {
		singleton = &permissions{}
	})
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
		return fmt.Errorf("%s: error initializing permission's elasticsearch dao: %v", logTag, err)
	}

	return nil
}

func (p *permissions) Routes() []plugin.Route {
	return p.routes()
}
