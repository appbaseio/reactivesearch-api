package elasticsearch

import (
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

const (
	logTag = "[elasticsearch]"
)

var (
	singleton *elasticsearch
	once      sync.Once
)

type elasticsearch struct {
	specs          []api
	systemESClient *es7.Client
}

func Instance() *elasticsearch {
	once.Do(func() { singleton = &elasticsearch{} })
	return singleton
}

func (es *elasticsearch) Name() string {
	return logTag
}

func (es *elasticsearch) InitFunc(mw []middleware.Middleware) error {
	if util.MultiTenant {
		// Init the system ES client
		var clientErr error
		es.systemESClient, clientErr = util.GetSystemClient()
		if clientErr != nil {
			return clientErr
		}
		// Cache the indexes for the tenants
		indexCacheErr := es.InitCacheIndexes()
		if indexCacheErr != nil {
			return indexCacheErr
		}

		storageCacheErr := FetchStorageFromES()
		if storageCacheErr != nil {
			return storageCacheErr
		}

		// Add a cronjob to update it every 60seconds
		storageSyncCron := cron.New()
		storageSyncCron.AddFunc("@every 60s", func() {
			err := FetchStorageFromES()
			if err != nil {
				log.Warnln(logTag, ": ", err.Error())
			}
		})
		storageSyncCron.Start()
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
