package openai

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/middleware"
	"github.com/appbaseio-confidential/reactivesearch/plugins"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

const (
	logTag                  = "[open_ai]"
	defaultOpenAIEsIndex    = ".openai"
	defaultAIAnalyticsIndex = ".ai_analytics"
	defaultFAQIndex         = ".ai_faqs"
	envOpenAIEsIndex        = "OPENAI_ES_INDEX"
	typeName                = "_doc"
	openAIConfigDocID       = "openai_config"
	mapping                 = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d }, "mappings": {} }`
	analyticsMapping        = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d }, "mappings": { "properties": { "timestamp": { "type": "date" } } } }`
	FAQMapping              = `{ "settings": { %s "index.number_of_shards": 1, "index.number_of_replicas": %d }, "mappings": { "properties": { "updated_at": { "type": "long" } } } }`
	zincMapping             = `
	{
		"name": "%s",
		"storage_type": "disk",
		"mappings": {
			"properties": {
				"searchboxId": {
                    "type": "text",
                    "index": true,
                    "store": false,
                    "sortable": false,
                    "aggregatable": false,
                    "highlightable": false,
					"fields": {
						"keyword":{
							"type":"keyword",
							"ignore_above":256
						}
					}
                }
			}
		}
	}
	`
)

var (
	singleton *OpenAI
	once      sync.Once
)

// OpenAI plugin deals with managing query translation.
type OpenAI struct {
	sessionMap  *SessionIdToChatGPTResponse
	es          openaiService
	analyticsEs openaiAnalyticsService
	faqEs       openaiFAQServiceEs
	faqZinc     openaiFAQService
}

// Instance returns the singleton instance of the plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *OpenAI {
	once.Do(func() {
		singleton = &OpenAI{}
	})
	return singleton
}

// Name returns the name of the plugin: [open_ai]
func (r *OpenAI) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (r *OpenAI) InitFunc() error {
	// Initialize the chatGPT session ID map
	r.sessionMap = SessionInstance()

	openAIIndex := os.Getenv(envOpenAIEsIndex)
	if openAIIndex == "" {
		openAIIndex = defaultOpenAIEsIndex
	}

	// initialize the dao
	var err error
	r.es, err = initPlugin(openAIIndex, mapping)
	if err != nil {
		return err
	}

	// Initialize the analytics plugin as well
	r.analyticsEs, err = initAnalyticsPlugin(defaultAIAnalyticsIndex, analyticsMapping)
	if err != nil {
		return err
	}

	// Initialize the FAQ index on ES
	r.faqEs, err = initFAQPluginES(defaultFAQIndex, FAQMapping)
	if err != nil {
		return err
	}

	// Initiate the zinc index
	r.faqZinc, err = initFAQPlugin(defaultFAQIndex, zincMapping)
	if err != nil {
		return err
	}

	// Fetch the settings
	configFetched, configFetchErr := r.es.getSettings(context.Background())
	if configFetchErr != nil {
		log.Warn(logTag, ": error while fetching config with err: ", configFetchErr.Error())
		return configFetchErr
	}
	r.SetConfig(configFetched)

	// Set plugin cache sync script
	script := CacheSyncScript{
		index: openAIIndex,
	}
	util.AddSyncScript(script)

	// Do a sync for Zinc from ES and add a cron to do it
	// regularly
	syncErr := r.faqZinc.SyncFAQToZinc(defaultFAQIndex, defaultFAQIndex)
	if syncErr != nil {
		return syncErr
	}

	// Add the cronjob
	//
	// Note: Use UTC timezone and run at 23:30:00 to avoid conflicts with snapshots
	//
	// For self-hosted users, the cronjob will run every hour since AccAPI proxy
	// is not present there.

	isSelfHosted := util.Billing == "true"

	var cronjob *cron.Cron
	if isSelfHosted {
		// Set the cron interval to 1h
		cronjob = cron.New()
		cronjob.AddFunc("@every 1h", func() {
			err4 := r.faqZinc.SyncFAQToZinc(defaultFAQIndex, defaultFAQIndex)
			if err4 != nil {
				log.Errorln(logTag, ": sync process failed for FAQ suggestions, reason:", err4)
			}
		})
	} else {
		cronjob = cron.New(
			cron.WithLocation(time.UTC))
		cronjob.AddFunc("30 23 * * *", func() {
			err4 := r.faqZinc.SyncFAQToZinc(defaultFAQIndex, defaultFAQIndex)
			if err4 != nil {
				log.Errorln(logTag, ": sync process failed for FAQ suggestions, reason:", err4)
			}
		})
	}

	// Start the cronjob
	cronjob.Start()

	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (r *OpenAI) Routes() []plugins.Route {
	return r.routes()
}

func (r *OpenAI) ESMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}

// Default empty middleware array function
func (a *OpenAI) RSMiddleware() []middleware.Middleware {
	return make([]middleware.Middleware, 0)
}
