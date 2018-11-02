package logs

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/internal/errors"
)

const (
	logTag         = "[logs]"
	envEsURL       = "ES_CLUSTER_URL"
	envLogsEsIndex = "LOGS_ES_INDEX"
	mapping        = `{"settings":{"number_of_shards":3, "number_of_replicas":2}}`
)

var (
	singleton *Logs
	once      sync.Once
)

// Logs plugin records an elasticsearch request and its response.
type Logs struct {
	es *elasticsearch
}

// Instance returns the singleton instance of Logs plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Logs in order to avoid stateless instances of the plugin.
func Instance() *Logs {
	once.Do(func() {
		singleton = &Logs{}
	})
	return singleton
}

// Name returns the name of the plugin: "[logs]"
func (l *Logs) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (l *Logs) InitFunc() error {
	// fetch the required env vars
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envLogsEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envLogsEsIndex)
	}

	// initialize the elasticsearch client
	var err error
	l.es, err = newClient(url, indexName, mapping)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (l *Logs) Routes() []route.Route {
	return []route.Route{}
}
