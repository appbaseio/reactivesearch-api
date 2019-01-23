package users

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	logTag              = "[users]"
	envEsURL            = "ES_CLUSTER_URL"
	envUsersEsIndex     = "USERS_ES_INDEX"
	defaultUsersEsIndex = ".users"
	settings            = `{ "settings" : { "number_of_shards" : 3, "number_of_replicas" : %d } }`
)

var (
	singleton *users
	once      sync.Once
)

type users struct {
	es userService
}

func init() {
	arc.RegisterPlugin(instance())
}

// Use only this function to fetch the instance of user from within
// this package to avoid creating stateless duplicates of the plugin.
func instance() *users {
	once.Do(func() { singleton = &users{} })
	return singleton
}

func (u *users) Name() string {
	return logTag
}

func (u *users) InitFunc() error {
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

func (u *users) Routes() []route.Route {
	return u.routes()
}
