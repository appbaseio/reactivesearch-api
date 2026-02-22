package pipelines

import "context"

type pipelinesService interface {
	createPipeline(ctx context.Context, pipelineID string, record ESPipelineDoc) error
	updatePipeline(ctx context.Context, pipelineID string, record ESPipelineDoc) error
	deletePipeline(ctx context.Context, pipelineID string) error
	getPipelines(ctx context.Context) ([]ESPipelineDoc, error)
	getPipeline(ctx context.Context, pipelineID string) (*ESPipelineDoc, error)
	getPipelinesSize(ctx context.Context) (*int64, error)
}

type pipelineInvocationService interface {
	createInvocationRecord(ctx context.Context, record PipelineInvoke) error
	createPipelineInvokeRecord(pipelineIDPassed string, version int, stages *map[string]PipelineInvokeStage, took int) error
	queryPipelinesUsage(ctx context.Context, from, to string, size int, filters map[string]interface{}) ([]byte, error)
	queryPipelineStageUsage(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error)
	queryPipelineVersionUsage(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error)
	queryPipelineVersionTimeTaken(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error)
	queryPipelineVersionErrorRate(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error)
	queryPipelineVersionStageTimeTaken(ctx context.Context, from, to string, size int, pipelineID string, version int, filters map[string]interface{}) ([]byte, error)
	queryPipelineVersionStageErrorRate(ctx context.Context, from, to string, size int, pipelineID string, version int, filters map[string]interface{}) ([]byte, error)
	queryPipelineVersionStats(ctx context.Context, pipelineID string) (map[int]interface{}, error)
	rolloverIndexJob(alias string)
}

type pipelineLogService interface {
	getPipelineLogs(ctx context.Context, logsFilter logsFilter) ([]byte, error)
	getPipelineLogById(ctx context.Context, logId string, parseDiffs bool) ([]byte, *Error)
	rolloverIndexJob(alias string)
}

type pipelineVarService interface {
	createVar(ctx context.Context, varId string, varDoc PipelineVar) error
	updateVar(ctx context.Context, varId string, varDoc PipelineVar) error
	deleteVar(ctx context.Context, varId string, key string) error
	getVars(ctx context.Context) ([]PipelineVar, error)
}
