package rules

import (
	"context"
	"os"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"go.kuoruan.net/v8go-polyfills/base64"
	"rogchap.com/v8go"
)

const (
	logTag                = "[rules]"
	defaultRulesEsIndex   = ".rules"
	typeName              = "_doc"
	envRulesEsIndexSuffix = "RULES_ES_INDEX_SUFFIX"
	defaultSettingsRuleID = "settings_rule"
	mapping               = `{"mappings": %s, "settings":{ %s "index.number_of_shards":1,"index.number_of_replicas":%d}}`
)

var (
	singleton *Rules
	once      sync.Once
)

// Store details about all the running cron jobs
type RunningJob struct {
	RuleId  *string
	CronJob *cron.Cron
}

// Rules plugin deals with managing query rules.
type Rules struct {
	lock        sync.Mutex
	es          rulesService
	iso         *v8go.Isolate
	context     *v8go.Context
	runningJobs []RunningJob
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
	indexPrefix := os.Getenv(envRulesEsIndexSuffix)
	if indexPrefix == "" {
		indexPrefix = defaultRulesEsIndex
	}
	indexPrefix = util.MetaIndexName(indexPrefix)

	var esRules []ESRuleDoc
	if util.ShouldCreateMetaIndex(util.MetaIndexRules) {
		var err error
		r.es, err = initPlugin(indexPrefix, mapping)
		if err != nil {
			return err
		}
		util.AddMigrationScript(MappingsMigration{
			es:        r.es.(*elasticsearch),
			indexName: indexPrefix,
		})
		esRules, err = r.es.getRules(context.Background())
		if err != nil {
			log.Errorln(logTag, ":", "error while retrieving the rules:", err)
			return nil
		}
		SetRulesToCache(esRules)
		util.AddSyncScript(CacheSyncScript{index: indexPrefix})
	} else {
		log.Infoln(logTag, ": skipping ES index creation (setup profile:", util.GetSetupProfile(), ")")
	}

	// instantiate a new JavaScript VM
	r.iso = v8go.NewIsolate()
	global := v8go.NewObjectTemplate(r.iso)

	// Inject fetch support
	fetchfn := r.getFetchFn()
	global.Set("fetch", fetchfn, v8go.ReadOnly)

	// Inject base64 support
	baseErr := base64.InjectTo(r.iso, global)
	if baseErr != nil {
		log.Warnln(logTag, "error while injecting base64 support, ", baseErr)
	}

	// Instantiate for the first time
	r.ReInstantiateV8Context(global)

	// init cron job to reinstantiate the the V8 ISO every min
	cronjob := cron.New()
	cronjob.AddFunc("@every 60m", func() {
		r.lock.Lock()
		r.ReInstantiateV8Context(global)
		r.lock.Unlock()
	})
	cronjob.Start()

	// Start the cron rules
	if r.es != nil {
		r.initCronRules(esRules)
	}

	return nil
}

// ReInstantiateV8Context cleans up any heap associated with the existing
// context and reinstantiates it
func (r *Rules) ReInstantiateV8Context(global *v8go.ObjectTemplate) {
	log.Debugln(logTag, ": re-instantiating v8 context")
	// cleanup the existing context
	if r.context != nil {
		r.context.Close()
	}
	// global context to execute script
	r.context = v8go.NewContext(r.iso, global)

	// load all the global scripts
	_, scriptError := r.context.RunScript(compromiseScript, "compromise.js")
	if scriptError != nil {
		log.Errorln("error loading compromise script", scriptError)
	}
	_, scriptError = r.context.RunScript(compromiseDatesScript, "compromise-dates.js")
	if scriptError != nil {
		log.Errorln("error loading compromise dates script", scriptError)
	}
	_, scriptError = r.context.RunScript(compromiseNumbersScript, "compromise-numbers.js")
	if scriptError != nil {
		log.Errorln("error loading compromise numbers script", scriptError)
	}
	_, scriptError = r.context.RunScript(`nlp.extend(compromiseNumbers).extend(compromiseDates)`, "compromise-extend.js")
	if scriptError != nil {
		log.Errorln("error extending compromise to include dates and numbers", scriptError)
	}
	_, scriptError = r.context.RunScript(lodashScript, "lodash.js")
	if scriptError != nil {
		log.Errorln("error loading lodash script", scriptError)
	}
	_, scriptError = r.context.RunScript(cryptoScript, "crypto.js")
	if scriptError != nil {
		log.Errorln("error loading crypto script", scriptError)
	}
	_, scriptError = r.context.RunScript(mongoDBQueryTranslateScript, "mongoDB.js")
	if scriptError != nil {
		log.Errorln("error loading mongoDB script", scriptError)
	}
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (r *Rules) Routes() []plugins.Route {
	return r.routes()
}

func (r *Rules) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		r.saveRequestToCtx,
		r.intercept,
	}
}

func (r *Rules) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{
		r.saveRequestToCtx,
		r.intercept,
	}
}

// Expose plugin specific routes
func (r *Rules) AlternateRoutes() []plugins.Route {
	return make([]plugins.Route, 0)
}
