package analytics

import (
	"fmt"
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
	analyticsType "github.com/appbaseio-confidential/arc/internal/types/analytics"
)

const (
	logTag              = "[analytics]"
	envEsURL            = "ES_CLUSTER_URL"
	envAnalyticsEsIndex = "ANALYTICS_ES_INDEX"
)

var (
	instance *analytics
	once     sync.Once
)

type analytics struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(Instance())
}

func Instance() *analytics {
	once.Do(func() {
		instance = &analytics{}
	})
	return instance
}

// Name returns the name of the plugin: 'analytics'.
func (a *analytics) Name() string {
	return logTag
}

// InitFunc reads the required environment variables and initializes
// the elasticsearch as its dao. The function returns EnvVarNotSetError
// in case the required environment variables are not set before the plugin
// is loaded.
func (a *analytics) InitFunc() error {
	// fetch the required env vars
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envAnalyticsEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envAnalyticsEsIndex)
	}
	mapping := analyticsType.IndexMapping

	// initialize the dao
	var err error
	a.es, err = NewES(url, indexName, mapping)
	if err != nil {
		return fmt.Errorf("%s: error initializing analytics' elasticsearch dao: %v", logTag, err)
	}

	return nil
}

// Routes returns the endpoints associated with analytics.
func (a *analytics) Routes() []plugin.Route {
	return a.routes()
}
