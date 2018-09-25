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
	envUserEsURL   = "USER_ES_URL"
	envUserEsIndex = "USER_ES_INDEX"
	envUserEsType  = "USER_ES_TYPE"
)

// TODO: Justify the API
type Users struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(New())
}

func New() *Users {
	return &Users{}
}

func (u *Users) Name() string {
	return pluginName
}

func (u *Users) InitFunc() error {
	log.Printf("%s: initializing plugin: %s", logTag, pluginName)

	// fetch vars from env
	esURL := os.Getenv(envUserEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envUserEsURL)
	}
	indexName := os.Getenv(envUserEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envUserEsIndex)
	}
	typeName := os.Getenv(envUserEsType)
	if typeName == "" {
		return errors.NewEnvVarNotSetError(envUserEsType)
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
