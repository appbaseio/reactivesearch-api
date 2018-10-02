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
	pluginName      = "users"
	logTag          = "[users]"
	envEsURL        = "ES_CLUSTER_URL"
	envUsersEsIndex = "USERS_ES_INDEX"
	envUsersEsType  = "USERS_ES_TYPE"
)

type Users struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(&Users{})
}

// Name returns the name of the plugin: 'users'.
func (u *Users) Name() string {
	return pluginName
}

// InitFunc reads the required environment variables and initializes
// the elasticsearch as its dao. The function returns EnvVarNotSetError
// in case the required environment variables are not set before the plugin
// is loaded.
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

// Routes returns the routes that this plugin handles.
func (u *Users) Routes() []plugin.Route {
	return u.routes()
}
