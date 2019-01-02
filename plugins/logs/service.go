package logs

import "context"

type logsService interface {
	getRawLogs(ctx context.Context, from, size string, indices ...string) ([]byte, error)
	indexRecord(ctx context.Context, r record)
}