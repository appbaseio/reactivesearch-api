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
	envUserEsIndex       = "USER_ES_INDEX"
	envUserEsType        = "USER_ES_TYPE"
	envPermissionEsIndex = "PERMISSION_ES_INDEX"
	envPermissionEsType  = "PERMISSION_ES_TYPE"
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
	userIndex := os.Getenv(envUserEsIndex)
	if userIndex == "" {
		return errors.NewEnvVarNotSetError(envUserEsIndex)
	}
	userType := os.Getenv(envUserEsType)
	if userType == "" {
		return errors.NewEnvVarNotSetError(envUserEsType)
	}
	permissionIndex := os.Getenv(envPermissionEsIndex)
	if permissionIndex == "" {
		return errors.NewEnvVarNotSetError(envPermissionEsIndex)
	}
	permissionType := os.Getenv(envPermissionEsType)
	if permissionType == "" {
		return errors.NewEnvVarNotSetError(envPermissionEsType)
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
