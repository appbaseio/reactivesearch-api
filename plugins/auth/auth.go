package auth

import (
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
)

const (
	pluginName           = "auth"
	logTag               = "[auth]"
	envEsURL             = "ES_CLUSTER_URL"
	envUsersEsIndex       = "USERS_ES_INDEX"
	envUsersEsType        = "USERS_ES_TYPE"
	envPermissionsEsIndex = "PERMISSIONS_ES_INDEX"
	envPermissionsEsType  = "PERMISSIONS_ES_TYPE"
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
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	userIndex := os.Getenv(envUsersEsIndex)
	if userIndex == "" {
		return errors.NewEnvVarNotSetError(envUsersEsIndex)
	}
	userType := os.Getenv(envUsersEsType)
	if userType == "" {
		return errors.NewEnvVarNotSetError(envUsersEsType)
	}
	permissionIndex := os.Getenv(envPermissionsEsIndex)
	if permissionIndex == "" {
		return errors.NewEnvVarNotSetError(envPermissionsEsIndex)
	}
	permissionType := os.Getenv(envPermissionsEsType)
	if permissionType == "" {
		return errors.NewEnvVarNotSetError(envPermissionsEsType)
	}

	// initialize the dao
	var err error
	a.es, err = NewES(esURL, userIndex, userType, permissionIndex, permissionType)
	if err != nil {
		return err
	}

	return nil
}

func (a *Auth) Routes() []plugin.Route {
	return []plugin.Route{}
}
