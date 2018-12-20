package logs

type logsService interface {
	getRawLogs(from, size string, indices ...string) ([]byte, error)
	indexRecord(r record)
}