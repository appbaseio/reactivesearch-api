package analytics

import (
	"fmt"
	"log"
	"os"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
	"github.com/appbaseio-confidential/arc/internal/types/analytics"
)

const (
	pluginName          = "analytics"
	logTag              = "[analytics]"
	envEsURL            = "ES_CLUSTER_URL"
	envAnalyticsEsIndex = "ANALYTICS_ES_INDEX"
	envAnalyticsEsType  = "ANALYTICS_ES_TYPE"
)

type Analytics struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(&Analytics{})
}

// Name returns the name of the plugin: 'analytics'.
func (a *Analytics) Name() string {
	return pluginName
}

// InitFunc reads the required environment variables and initializes
// the elasticsearch as its dao. The function returns EnvVarNotSetError
// in case the required environment variables are not set before the plugin
// is loaded.
func (a *Analytics) InitFunc() error {
	log.Printf("%s: initializing plugin: %s\n", logTag, pluginName)

	// fetch the required env vars
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envAnalyticsEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envAnalyticsEsIndex)
	}
	typeName := os.Getenv(envAnalyticsEsType)
	if typeName == "" {
		return errors.NewEnvVarNotSetError(envAnalyticsEsType)
	}
	mapping := analytics.IndexMapping

	// initialize the dao
	var err error
	a.es, err = NewES(url, indexName, typeName, mapping)
	if err != nil {
		return fmt.Errorf("%s: error initializing analytics' elasticsearch dao: %v", logTag, err)
	}

	return nil
}

func (a *Analytics) Routes() []plugin.Route {
	return a.routes()
}
