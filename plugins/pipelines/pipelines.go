package pipelines

import (
	"context"
	"os"
	"sync"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	logTag                            = "[pipelines]"
	defaultPipelinesEsIndex           = ".pipelines"
	defaultPipelinesLogEsIndex        = ".pipeline_logs"
	defaultPipelineInvocationIndex    = ".pipeline_invocations"
	defaultPipelineVarsIndex          = ".pipeline_vars"
	typeName                          = "_doc"
	envPipelinesEsIndexSuffix         = "PIPELINES_ES_INDEX_SUFFIX"
	mapping                           = `{"mappings": %s, "settings":{ %s "index.number_of_shards": %d, "index.number_of_replicas":%d}}`
	envPipelineLogFilePath            = "PIPELINE_LOG_FILE_PATH"
	defaultPipelineLogFilePath        = "log/arc/pipeline.json"
	envPipelineInvocationFilePath     = "PIPELINE_INVOCATION_FILE_PATH"
	defaultPipelineInvocationFilePath = "log/arc/pipeline_invocation.json"
	XCacheHeader                      = "x-cache"
	XPipelineID                       = "x-pipeline-id"
	rolloverConfig                    = `{"max_age":  "%s", "max_docs": %d, "max_size": "%s"}`
	invocationConfig                  = `
	{
	  "aliases": {
		"%s": {
		  "is_write_index": true
	    }
	  },
	  "settings": {
		%s
	    "index.number_of_shards": %d,
	    "index.number_of_replicas": %d
	  },
	  "mappings": %s
	}`
	logsConfig = `
	{
	  "aliases": {
		"%s": {
		  "is_write_index": true
	    }
	  },
	  "settings": {
		%s
	    "index.number_of_shards": %d,
	    "index.number_of_replicas": %d
	  },
	  "mappings": %s
	}`
)

// Pipelines plugin deals with managing user defined pipelines.
var (
	singleton *Pipelines
	once      sync.Once
)

// Pipelines plugin deals with managing pipelines.
type Pipelines struct {
	es pipelinesService

	// Store the invocation ES to store invocation records
	invocationEs pipelineInvocationService

	// Pipeline map to store map of paths to the pipelineId's
	PathToPipeline map[string][]*ESPipelineDoc

	// Store the logs ES connector
	logEs pipelineLogService

	// Store the vars ES connector
	varEs pipelineVarService

	// Lumberjack to write logs
	lumberjack lumberjack.Logger

	// Lumberjack to write pipeline invocations
	invocationLumberjack lumberjack.Logger

	pipelineSchema []byte

	validateSession *ValidateIDToConsoleLogs
}

// Instance returns the singleton instance of the plugin. Instance
// should be the only way (both within or outside the package) to fetch
// the instance of the plugin, in order to avoid stateless duplicates.
func Instance() *Pipelines {
	once.Do(func() { singleton = &Pipelines{} })
	return singleton
}

// Name returns the name of the plugin: [pipelines]
func (r *Pipelines) Name() string {
	return logTag
}

// InitFunc initializes the dao, i.e. elasticsearch client, and should be executed
// only once in the lifetime of the plugin.
func (p *Pipelines) InitFunc() error {
	// If the createSchema flag is passed, create the schema
	// and don't do anything else.
	if util.CreateSchema {
		// Create the schema and return
		return createPipelineSchema()
	}

	indexPrefix := os.Getenv(envPipelinesEsIndexSuffix)
	if indexPrefix == "" {
		indexPrefix = defaultPipelinesEsIndex
	}
	indexPrefix = util.MetaIndexName(indexPrefix)

	// initialize the dao
	var err error
	if util.ShouldCreateMetaIndex(util.MetaIndexPipelines) {
		p.es, err = initPlugin(indexPrefix, mapping)
		if err != nil {
			return err
		}
	}

	if util.ShouldCreateMetaIndex(util.MetaIndexPipelineInvocations) {
		p.invocationEs, err = initInvocationIndex(util.MetaIndexName(defaultPipelineInvocationIndex), mapping)
		if err != nil {
			return err
		}
	}

	if util.ShouldCreateMetaIndex(util.MetaIndexPipelineLogs) {
		p.logEs, err = initLogIndex(util.MetaIndexName(defaultPipelinesLogEsIndex), logsConfig)
		if err != nil {
			return err
		}
	}

	if util.ShouldCreateMetaIndex(util.MetaIndexPipelineVars) {
		p.varEs, err = initVarIndex(util.MetaIndexName(defaultPipelineVarsIndex), mapping)
		if err != nil {
			return err
		}
	}

	// Init the validateId to logs session single
	p.validateSession = ValidateSessionOnce()

	// Init the lumberjack instance so it can be used
	filePath := os.Getenv(envPipelineLogFilePath)
	if filePath == "" {
		log.Warnln(logTag, envPipelineLogFilePath+" is not defined log will get stored at ", defaultPipelineLogFilePath)
		filePath = defaultPipelineLogFilePath
	}
	// configure lumberjack
	p.lumberjack = lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    1000,
		MaxBackups: 3,
		MaxAge:     30, //days
	}

	// Init the lumberjack instance for pipeline invocations as well
	invocationFilePath := os.Getenv(envPipelineInvocationFilePath)
	if invocationFilePath == "" {
		log.Warnln(logTag, envPipelineInvocationFilePath+" is not defined, log will get stored at: ", defaultPipelineInvocationFilePath)
		invocationFilePath = defaultPipelineInvocationFilePath
	}

	// Configure lumberjack for pipeline invocations
	p.invocationLumberjack = lumberjack.Logger{
		Filename:   invocationFilePath,
		MaxSize:    1000,
		MaxBackups: 3,
		MaxAge:     30, // in days
	}

	if p.es != nil {
		util.AddMigrationScript(MappingsMigration{
			es:        p.es.(*elasticsearch),
			indexName: indexPrefix,
		})
	}
	if p.es != nil {
		esRules, err := p.es.getPipelines(context.Background())
		if err != nil {
			log.Errorln(logTag, ":", "error while retrieving the pipelines:", err)
			return nil
		}
		SetPipelinesToCache(esRules)
	}

	if p.varEs != nil {
		pipelineVars, err := p.varEs.getVars(context.Background())
		if err != nil {
			log.Errorln(logTag, ": error while retrieving the pipeline variables, ", err)
			return nil
		}
		SetVars(pipelineVars)
	}

	if p.es != nil {
		util.AddSyncScript(CacheSyncScript{index: indexPrefix})
	}

	// generate pipeline schema
	schema, err := GetPipelineSchema()
	if err != nil {
		log.Errorln(logTag, ":", "error while generating the pipeline schema:", err)
		return nil
	}
	// set schema
	p.pipelineSchema = schema

	if p.logEs != nil || p.invocationEs != nil {
		pipelineLogsIndex := util.MetaIndexName(defaultPipelinesLogEsIndex)
		pipelineInvocationsIndex := util.MetaIndexName(defaultPipelineInvocationIndex)
		cronjob := cron.New()
		if p.logEs != nil {
			cronjob.AddFunc("@midnight", func() { p.logEs.rolloverIndexJob(pipelineLogsIndex) })
			cronjob.AddFunc("@hourly", func() { p.logEs.rolloverIndexJob(pipelineLogsIndex) })
		}
		if p.invocationEs != nil {
			cronjob.AddFunc("@midnight", func() { p.invocationEs.rolloverIndexJob(pipelineInvocationsIndex) })
			cronjob.AddFunc("@hourly", func() { p.invocationEs.rolloverIndexJob(pipelineInvocationsIndex) })
		}
		cronjob.Start()
	}

	return nil
}

// Routes returns an empty slices since the plugin solely acts as a middleware.
func (p *Pipelines) Routes() []plugins.Route {
	return p.routes()
}

func (p *Pipelines) ESMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

func (p *Pipelines) RSMiddleware() []middleware.Middleware {
	return []middleware.Middleware{}
}

// Expose plugin specific routes
func (p *Pipelines) AlternateRoutes() []plugins.Route {
	// NOTE: Priority for routes will be handled in the handler
	// so no need to sort here.
	return p.alternateRoutes()
}
