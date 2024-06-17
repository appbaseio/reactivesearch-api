package searchrelevancy

import "context"

type searchRelevancyService interface {
	putSearchRelevancySettings(ctx context.Context, docID string, record SearchRelevancyStruct) error
	deleteSearchRelevancySettings(ctx context.Context, docID string) error
	getSearchRelevancySettings(ctx context.Context) (map[string]SearchRelevancyStruct, error)
}
