package analytics

import (
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/plugin"
	"github.com/appbaseio-confidential/arc/internal/errors"
	analyticsIndex "github.com/appbaseio-confidential/arc/internal/types/analytics"
)

const (
	logTag              = "[analytics]"
	envEsURL            = "ES_CLUSTER_URL"
	envAnalyticsEsIndex = "ANALYTICS_ES_INDEX"
)

var (
	instance *Analytics
	once     sync.Once
)

// Analytics plugin records and serves basic index or cluster level analytics.
type Analytics struct {
	es *elasticsearch
}

func init() {
	arc.RegisterPlugin(Instance())
}

// Instance returns the singleton instace of Analytics plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance analytics in order to avoid stateless instances of the plugin.
func Instance() *Analytics {
	once.Do(func() {
		instance = &Analytics{}
	})
	return instance
}

// Name is a part of Plugin interface that returns the name of the plugin: '[analytics]'.
func (a *Analytics) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (a *Analytics) InitFunc() error {
	// fetch the required env vars
	url := os.Getenv(envEsURL)
	if url == "" {
		return errors.NewEnvVarNotSetError(envEsURL)
	}
	indexName := os.Getenv(envAnalyticsEsIndex)
	if indexName == "" {
		return errors.NewEnvVarNotSetError(envAnalyticsEsIndex)
	}
	mapping := analyticsIndex.IndexMapping

	// initialize the dao
	var err error
	a.es, err = newClient(url, indexName, mapping)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns the analytics routes that the plugin serves.
func (a *Analytics) Routes() []plugin.Route {
	return a.routes()
}
