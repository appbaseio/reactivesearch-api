package auth

import (
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
)

const (
	pluginName     = "auth"
	logTag         = "[auth]"
	envUserEsURL   = "USER_ES_URL"
	envUserEsIndex = "USER_ES_INDEX"
	envUserEsType  = "USER_ES_TYPE"
)

var a *Auth

type Auth struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(Instance())
}

func Instance() *Auth {
	if a == nil {
		a = &Auth{}
	}
	return a
}

func (a *Auth) Name() string {
	return pluginName
}

func (a *Auth) InitFunc() error {
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

	// initialize the dao
	var err error
	a.es, err = NewES(esURL, indexName, typeName)
	if err != nil {
		return err
	}

	return nil
}

func (a *Auth) Routes() []plugin.Route {
	return []plugin.Route{}
}
