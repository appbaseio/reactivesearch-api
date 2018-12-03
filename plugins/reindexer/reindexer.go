package reindexer

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	logTag   = "[reindexer]"
	envEsURL = "ES_CLUSTER_URL"
)

var (
	singleton *reindexer
	once      sync.Once
)

func init() {
	arc.RegisterPlugin(instance())
}

type reindexer struct {
	es *elasticsearch
}

// Use only this function to fetch the instance of user from within
// this package to avoid creating stateless duplicates of the plugin.
func instance() *reindexer {
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

func (rx *reindexer) Routes() []route.Route {
	return rx.routes()
}
