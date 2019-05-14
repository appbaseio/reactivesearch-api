package users

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	logTag              = "[users]"
	envEsURL            = "ES_CLUSTER_URL"
	envUsersEsIndex     = "USERS_ES_INDEX"
	defaultUsersEsIndex = ".users"
	settings            = `{ "settings" : { "number_of_shards" : %d, "number_of_replicas" : %d } }`
)

var (
	singleton *Users
	once      sync.Once
)

// Users plugin deals with user management.
type Users struct {
	es userService
}

// Use only this function to fetch the instance of user from within
// this package to avoid creating stateless duplicates of the plugin.
// However, instance of Users is not meant to be used outside the package.
func Instance() *Users {
	once.Do(func() { singleton = &Users{} })
	return singleton
}

// Name is the implementation of Plugin interface.
func (u *Users) Name() string {
	return logTag
}

// InitFunc is the implementation of Plugin interface.
func (u *Users) InitFunc() error {
	// fetch vars from env
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envUsersEsIndex)
	if indexName == "" {
		indexName = defaultUsersEsIndex
	}

	// initialize the dao
	var err error
	u.es, err = newClient(esURL, indexName, settings)
	if err != nil {
		return err
	}

	return nil
}

// Routes is the implementation of plugin interface.
func (u *Users) Routes() []route.Route {
	return u.routes()
}
