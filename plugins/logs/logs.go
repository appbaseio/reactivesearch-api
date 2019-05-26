package logs

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/plugins"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	logTag             = "[logs]"
	defaultLogsEsIndex = ".logs"
	envEsURL           = "ES_CLUSTER_URL"
	envLogsEsIndex     = "LOGS_ES_INDEX"
	config             = `
	{
	  "mappings": {
	    "_doc": {
	      "properties": {
	        "category": { "type": "keyword" },
	        "request": { "type": "object" },
	        "response": { "type": "object" },
	        "timestamp": { "type": "date" }
	      }
	    }
	  },
	  "settings": {
	    "number_of_shards": %d,
	    "number_of_replicas": %d
	  }
	}`
)

var (
	singleton *Logs
	once      sync.Once
)

// Logs plugin records an elasticsearch request and its response.
type Logs struct {
	es logsService
}

// Instance returns the singleton instance of Logs plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Logs in order to avoid stateless instances of the plugin.
func Instance() *Logs {
	once.Do(func() { singleton = &Logs{} })
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
		indexName = defaultLogsEsIndex
	}

	// initialize the elasticsearch client
	var err error
	l.es, err = newClient(url, indexName, config)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (l *Logs) Routes() []plugins.Route {
	return l.routes()
}
