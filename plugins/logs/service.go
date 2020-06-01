package logs

import "context"

type logsService interface {
	getRawLogs(ctx context.Context, offset, startDate, endDate string, size int, filter string, indices ...string) ([]byte, error)
	indexRecord(ctx context.Context, r record)
	rolloverIndexJob(alias string)
}
