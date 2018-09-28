package permissions

import (
	"fmt"
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
)

const (
	pluginName           = "permissions"
	logTag               = "[permissions]"
	envEsURL             = "ES_CLUSTER_URL"
	envPermissionEsIndex = "PERMISSIONS_ES_INDEX"
	envPermissionEsType  = "PERMISSIONS_ES_TYPE"
)

type Permissions struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(&Permissions{})
}

func (p *Permissions) Name() string {
	return pluginName
}

func (p *Permissions) InitFunc() error {
	log.Printf("%s: initializing plugin: %s\n", logTag, pluginName)

	// fetch vars from env
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envPermissionEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envPermissionEsIndex)
	}
	typeName := os.Getenv(envPermissionEsType)
	if typeName == "" {
		return errors.NewEnvVarNotSetError(envPermissionEsType)
	}
	mapping := permission.IndexMapping

	// initialize the dao
	var err error
	p.es, err = NewES(url, indexName, typeName, mapping)
	if err != nil {
		return fmt.Errorf("%s: error initializing permission's elasticsearch dao: %v", logTag, err)
	}

	return nil
}

func (p *Permissions) Routes() []plugin.Route {
	return p.routes()
}
