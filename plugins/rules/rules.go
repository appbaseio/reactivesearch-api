package rules

import (
	"os"
	"sync"
	"net/http"

	"github.com/appbaseio-confidential/arc/plugins"
	"github.com/appbaseio-confidential/arc/middleware"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	logTag                = "[rules]"
	defaultRulesEsIndex   = "rules"
	envEsURL              = "ES_CLUSTER_URL"
	envRulesEsIndexSuffix = "RULES_ES_INDEX_SUFFIX"
	indexConfig           = `
	{
	  "mappings": {
	    "_doc": {
	      "properties": {
	        "query": { "type": "percolator" },
	        "if": {
	          "properties": {
	            "query": { "type": "keyword" },
	            "operator": { "type": "text" }
	          }
	        },
	        "then": {
	          "properties": {
	            "operator": { "type": "text" },
	            "payload": { "type": "object" }
	          }
	        }
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
	singleton *Rules
	once      sync.Once
)

// Rules plugin deals with managing query rules.
type Rules struct {
	es rulesService
}


// Instance returns the singleton instance of the plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *Rules {
	once.Do(func() { singleton = &Rules{} })
	return singleton
}

// Name returns the name of the plugin: [rules]
func (r *Rules) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (r *Rules) InitFunc(_ [] middleware.Middleware) error {
	// fetch vars from env
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexPrefix := os.Getenv(envRulesEsIndexSuffix)
	if indexPrefix == "" {
		indexPrefix = defaultRulesEsIndex
	}

	// initialize the dao
	var err error
	r.es, err = newClient(esURL, indexPrefix, indexConfig)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (r *Rules) Routes() []plugins.Route {
	return r.routes()
}

func (r *Rules) ESMiddleware() []middleware.Middleware {
	return [] middleware.Middleware {
		func(h http.HandlerFunc) http.HandlerFunc {
			return r.intercept(h)
		},
	}
}
