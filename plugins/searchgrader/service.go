package searchgrader

import "context"

type searchGraderService interface {
	updateGrade(ctx context.Context, record ESRecord) error
	getMetrics(ctx context.Context, record GradeMetricsRequest) (*GradeMetricsResponse, *int, error)
	getDocuments(ctx context.Context, query string) (map[string]interface{}, error)
}
