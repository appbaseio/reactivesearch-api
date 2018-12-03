package auth

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/errors"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
)

const (
	logTag                = "[auth]"
	envEsURL              = "ES_CLUSTER_URL"
	envUsersEsIndex       = "USERS_ES_INDEX"
	envPermissionsEsIndex = "PERMISSIONS_ES_INDEX"
)

var (
	singleton *Auth
	once      sync.Once
)

// Auth (TODO - clear cache after fixed entries: LRU?)
type Auth struct {
	mu               sync.Mutex
	usersCache       map[string]*user.User
	permissionsCache map[string]*permission.Permission
	es               *elasticsearch
}

func init() {
	arc.RegisterPlugin(Instance())
}

// Instance returns the singleton instance of the auth plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *Auth {
	once.Do(func() {
		singleton = &Auth{
			usersCache:       make(map[string]*user.User),
			permissionsCache: make(map[string]*permission.Permission),
		}
	})
	return singleton
}

// Name returns the name of the plugin: [auth]
func (a *Auth) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (a *Auth) InitFunc() error {
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
	a.es, err = newClient(esURL, userIndex, permissionIndex)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (a *Auth) Routes() []route.Route {
	return []route.Route{}
}
