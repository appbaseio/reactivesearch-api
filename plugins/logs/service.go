package logs

import "context"

type logsService interface {
	getRawLogs(ctx context.Context, logsConfig logsConfig) ([]byte, error)
	indexRecord(ctx context.Context, r record)
	rolloverIndexJob(alias string)
}
