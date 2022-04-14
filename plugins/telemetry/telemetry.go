package telemetry

import (
	"os"
	"strconv"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	badger "github.com/outcaste-io/badger/v3"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

const (
	logTag                   = "[telemetry]"
	eventType                = "telemetry" // New relic event name
	frontEndHeader           = "X-Search-Client"
	telemetryHeader          = "X-Enable-Telemetry"
	defaultServerMode        = "OSS"
	syncInterval             = 10  // interval in minutes to sync telemetry records
	deltaInterval            = 100 // in ms
	totalEventsPerRequest    = 2000
	defaultTelemetryFilePath = "/var/log/arc/telemetry"
	envTelemetryFilePath     = "TELEMETRY_FILE_PATH" // Just for local testing
)

var blacklistRoutes = []string{"/"}

var (
	singleton *Telemetry
	once      sync.Once
)

// Telemetry plugin records the API usage.
type Telemetry struct {
	filePath string
	db       *badger.DB
}

// Instance returns the singleton instance of Telemetry plugin.
// Note: Only this function must be used (both within and outside the package) to
// obtain the instance Logs in order to avoid stateless instances of the plugin.
func Instance() *Telemetry {
	once.Do(func() { singleton = &Telemetry{} })
	return singleton
}

// Name returns the name of the plugin: "[telemetry]"
func (t *Telemetry) Name() string {
	return logTag
}

// InitFunc is a part of Plugin interface that gets executed only once, and initializes
// the dao, i.e. elasticsearch before the plugin is operational.
func (t *Telemetry) InitFunc() error {

	filePath := os.Getenv(envTelemetryFilePath)
	if filePath == "" {
		log.Warnln(logTag, envTelemetryFilePath+" is not defined telemetry will get stored at ", defaultTelemetryFilePath)
		filePath = defaultTelemetryFilePath
	}
	t.filePath = filePath

	db, err := badger.Open(setBadgerOptions(filePath))
	if err != nil {
		log.Fatal(err)
	}
	t.db = db
	// Sync at the starting of arc
	t.syncTelemetryRecords()

	interval := "@every " + strconv.Itoa(syncInterval) + "m"

	cronjob := cron.New()
	cronjob.AddFunc(interval, t.syncTelemetryRecords)
	cronjob.Start()

	return nil
}

// Routes returns an empty slice of routes, since Logs is solely a middleware.
func (t *Telemetry) Routes() []plugins.Route {
	return []plugins.Route{}
}

// Default empty middleware array function
func (t *Telemetry) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (t *Telemetry) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
