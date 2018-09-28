package users

import (
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/user"
)

const (
	pluginName     = "users"
	logTag         = "[users]"
	envEsURL       = "ES_CLUSTER_URL"
	envUsersEsIndex = "USERS_ES_INDEX"
	envUsersEsType  = "USERS_ES_TYPE"
)

// TODO: Justify the API
type Users struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(&Users{})
}

func (u *Users) Name() string {
	return pluginName
}

func (u *Users) InitFunc() error {
	log.Printf("%s: initializing plugin: %s", logTag, pluginName)

	// fetch vars from env
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envUsersEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envUsersEsIndex)
	}
	typeName := os.Getenv(envUsersEsType)
	if typeName == "" {
		return errors.NewEnvVarNotSetError(envUsersEsType)
	}
	mapping := user.IndexMapping

	// initialize the dao
	var err error
	u.es, err = NewES(esURL, indexName, typeName, mapping)
	if err != nil {
		return err
	}

	return nil
}

func (u *Users) Routes() []plugin.Route {
	return u.routes()
}
