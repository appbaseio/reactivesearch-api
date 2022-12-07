package elasticsearch

import (
	"context"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
)

const (
	logTag         = "[elasticsearch]"
	systemESUrlKey = "SYSTEM_ES_URL"
)

var (
	singleton      *elasticsearch
	once           sync.Once
	systemESClient *es7.Client
)

type elasticsearch struct {
	specs []api
}

func Instance() *elasticsearch {
	once.Do(func() { singleton = &elasticsearch{} })
	return singleton
}

func (es *elasticsearch) Name() string {
	return logTag
}

func (es *elasticsearch) InitFunc(mw []middleware.Middleware) error {
	// Init the system ES client
	var clientErr error
	systemESClient, clientErr = initSystemESClient()
	if clientErr != nil {
		return clientErr
	}

	// Cache the indexes for the tenants
	indexCacheErr := CacheIndexesForTenants(systemESClient, context.Background())
	if indexCacheErr != nil {
		return indexCacheErr
	}

	return es.preprocess(mw)
}

func (es *elasticsearch) Routes() []plugins.Route {
	return es.routes()
}

// Default empty middleware array function
func (es *elasticsearch) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Enable plugin
func (es *elasticsearch) Enabled() bool {
	return util.IsExternalESRequired()
}
