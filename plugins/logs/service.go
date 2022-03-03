package logs

import "context"

type logsService interface {
	getRawLogs(ctx context.Context, logsFilter logsFilter) ([]byte, error)
	getRawLog(ctx context.Context, ID string) ([]byte, *LogError)
	indexRecord(ctx context.Context, r record)
	rolloverIndexJob(alias string)
}
