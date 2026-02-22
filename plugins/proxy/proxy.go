package proxy

import (
	"io/ioutil"
	"os"
	"sync"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

const (
	proxyTag                    = "[proxy]"
	arcUUID                     = "ARC_ID"
	clusterUUID                 = "CLUSTER_ID"
	defaultAppbaseCacheFilePath = "log/arc/plan.json"
	envAppbaseCachePath         = "PLAN_FILE_PATH"
	hashString                  = "APPBASE PLAN !!####@@@@@@@######!! APPBASE PLAN"
	logTag                      = "[proxy]"
)

var (
	singleton *Proxy
	once      sync.Once
)

// Proxy records an elasticsearch request and its response.
type Proxy struct {
	arcID     string
	clusterID string
	ap        proxyService
}

// Instance returns the singleton instance of Logs plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Logs in order to avoid stateless instances of the plugin.
func Instance() *Proxy {
	once.Do(func() { singleton = &Proxy{} })
	return singleton
}

// Name returns the name of the plugin: "[logs]"
func (p *Proxy) Name() string {
	return proxyTag
}

func writePlanToFile(planInfo []byte) {
	// transform to base64 format
	filePath := os.Getenv(envAppbaseCachePath)
	if filePath == "" {
		log.Warnln(logTag, envAppbaseCachePath+" is not defined Appbase cache will get stored at ", defaultAppbaseCacheFilePath)
		filePath = defaultAppbaseCacheFilePath
	}

	encryptedContent, err := encrypt(planInfo, hashString)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}
	// write to file
	err2 := ioutil.WriteFile(filePath, encryptedContent, 0644)
	if err2 != nil {
		log.Errorln(logTag, "error updating Appbase cache", err2)
		return
	}
}

func readPlanToFile() ([]byte, error) {
	// transform to base64 format
	filePath := os.Getenv(envAppbaseCachePath)
	if filePath == "" {
		log.Warnln(logTag, envAppbaseCachePath+" is not defined Appbase cache will get stored at ", defaultAppbaseCacheFilePath)
		filePath = defaultAppbaseCacheFilePath
	}
	// write to file
	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Errorln(logTag, "error updating Appbase cache")
		return nil, err
	}
	decryptedContent, err2 := decrypt(contents, hashString)
	if err2 != nil {
		log.Errorln(logTag, "error decrypting Appbase cache", err2)
		return nil, err2
	}
	return decryptedContent, nil
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (p *Proxy) InitFunc() error {
	if util.ClusterBilling == "true" {
		// fetch the required env vars
		clusterID := os.Getenv(clusterUUID)
		if clusterID == "" {
			return errors.NewEnvVarNotSetError(clusterUUID)
		}
		p.clusterID = clusterID
	} else if util.Billing == "true" && !util.OfflineBilling {
		// fetch the required env vars
		arcID, err := util.GetAppbaseID()
		if err != nil {
			return err
		}
		p.arcID = arcID
	} else if util.HostedBilling == "true" {
		// fetch the required env vars
		arcID := os.Getenv(clusterUUID)
		if arcID == "" {
			return errors.NewEnvVarNotSetError(clusterUUID)
		}
		p.arcID = arcID
	}
	// Register hook to write plan details to file system
	fn := writePlanToFile
	util.SetPlanDetailsHook(&fn)
	// Write plan details at init
	planDetails := util.GetPlanDetails()
	if planDetails != nil {
		writePlanToFile(*planDetails)
	}
	// initialize the elasticsearch client
	var err error
	p.ap, err = initPlugin(p.arcID, p.clusterID)
	if err != nil {
		return err
	}
	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (p *Proxy) Routes() []plugins.Route {
	return p.routes()
}

// ESMiddleware is a default empty middleware function
func (p *Proxy) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// RSMiddleware is a default empty middleware function
func (p *Proxy) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Expose plugin specific routes
func (p *Proxy) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
