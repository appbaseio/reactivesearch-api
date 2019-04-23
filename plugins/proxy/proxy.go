package proxy

import (
	"os"
	"sync"
	"log"
	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/route"
	"github.com/appbaseio-confidential/arc/errors"
)

const (
	proxyTag = "[proxy]"
	arcUUID  = "ARC_ID"
	subID    = "SUBSCRIPTION_ID"
	email	= "EMAIL"
)

var (
	singleton *Proxy
	once      sync.Once
)

// Logs plugin records an elasticsearch request and its response.
type Proxy struct {
	arcID string
	subID string
	email string
	ap proxyService
}

func init() {
	arc.RegisterPlugin(Instance())
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

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (p *Proxy) InitFunc() error {
	// fetch the required env vars
	arcID := os.Getenv(arcUUID)
	if arcID == "" {
		return errors.NewEnvVarNotSetError(arcUUID)
	}
	p.arcID = arcID
	emailID := os.Getenv(email)
	if emailID == "" {
		return errors.NewEnvVarNotSetError(emailID)
	}
	p.email = emailID
	subscriptionID := os.Getenv(subID)
	if subscriptionID == "" {
		log.Println("subscription ID no found. intializing in trial mode")
	}
	p.subID = subscriptionID
	// initialize the elasticsearch client
	var err error
	p.ap, err = newClient(arcID, subscriptionID, emailID)
	if err != nil {
		return err
	}

	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (p *Proxy) Routes() []route.Route {
	return p.routes()
}
