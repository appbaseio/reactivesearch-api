package reindexer

import (
	"os"
	"sync"

	"github.com/appbaseio/arc/plugins"
	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/errors"
)

const (
	logTag   = "[reindexer]"
	envEsURL = "ES_CLUSTER_URL"
)

var (
	singleton *reindexer
	once      sync.Once
)

type reindexer struct {
	es reindexService
}

// Use only this function to fetch the instance of user from within
// this package to avoid creating stateless duplicates of the plugin.
func Instance() *reindexer {
	once.Do(func() { singleton = &reindexer{} })
	return singleton
}

func (rx *reindexer) Name() string {
	return logTag
}

func (rx *reindexer) InitFunc() error {
	// fetch env vars
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}

	// initialize the dao
	var err error
	rx.es, err = newClient(esURL)
	if err != nil {
		return err
	}

	return nil
}

func (rx *reindexer) Routes() []plugins.Route {
	return rx.routes()
}

// Default empty middleware array function
func (rx *reindexer) ESMiddleware() [] middleware.Middleware {
	return make([] middleware.Middleware, 0)
}
