package rules

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	logTag                = "[rules]"
	envEsURL              = "ES_CLUSTER_URL"
	envRulesEsIndexSuffix = "RULES_ES_INDEX_SUFFIX"

	indexConfig = `
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
	    "number_of_shards": 3,
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

func init() {
	arc.RegisterPlugin(Instance())
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
func (r *Rules) InitFunc() error {
	// fetch vars from env
	esURL := os.Getenv(envEsURL)
	if esURL == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexPrefix := os.Getenv(envRulesEsIndexSuffix)
	if indexPrefix == "" {
		indexPrefix = "rules"
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
func (r *Rules) Routes() []route.Route {
	return r.routes()
}
