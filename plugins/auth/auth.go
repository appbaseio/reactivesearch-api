package auth

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/types/user"
)

const (
	logTag                = "[auth]"
	envEsURL              = "ES_CLUSTER_URL"
	envUsersEsIndex       = "USERS_ES_INDEX"
	envPermissionsEsIndex = "PERMISSIONS_ES_INDEX"
)

var (
	instance *auth
	once     sync.Once
)

// TODO: clear cache after fixed entries: LRU?
type auth struct {
	mu               sync.Mutex
	usersCache       map[string]*user.User
	permissionsCache map[string]*permission.Permission
	es               *elasticsearch
}

func init() {
	arc.RegisterPlugin(Instance())
}

func Instance() *auth {
	once.Do(func() {
		instance = &auth{
			usersCache:       make(map[string]*user.User),
			permissionsCache: make(map[string]*permission.Permission),
		}
	})
	return instance
}

func (a *auth) Name() string {
	return logTag
}

func (a *auth) InitFunc() error {
	// fetch vars from env
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	userIndex := os.Getenv(envUsersEsIndex)
	if userIndex == "" {
		return errors.NewEnvVarNotSetError(envUsersEsIndex)
	}
	permissionIndex := os.Getenv(envPermissionsEsIndex)
	if permissionIndex == "" {
		return errors.NewEnvVarNotSetError(envPermissionsEsIndex)
	}

	// initialize the dao
	var err error
	a.es, err = NewES(esURL, userIndex, permissionIndex)
	if err != nil {
		return err
	}

	return nil
}

func (a *auth) Routes() []plugin.Route {
	return []plugin.Route{}
}
