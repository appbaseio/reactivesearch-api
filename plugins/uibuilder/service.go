package uibuilder

import "context"

type uiBuilderService interface {
	savePreference(ctx context.Context, id string, record interface{}) error
	getSearchPreference(ctx context.Context, id string) (SearchPreference, error)
	getRecommendationPreference(ctx context.Context, id string) (RecommendationPreference, error)
	deletePreference(ctx context.Context, index string) error
	getSearchPreferences(ctx context.Context) ([]SearchPreference, error)
	getRecommendationPreferences(ctx context.Context) ([]RecommendationPreference, error)
	setAuthPreference(ctx context.Context, record interface{}) error
	getAuthPreference(ctx context.Context) (map[string]interface{}, error)
}

type searchboxService interface {
	saveSearchBox(ctx context.Context, searchboxId string, payload SearchBoxESModel) *Error
	getSearchBox(ctx context.Context, searchboxId string) (*SearchBoxESModel, *Error)
	getSearchBoxes(ctx context.Context, hidden bool) ([]SearchBoxESModel, *Error)
	deleteSearchBox(ctx context.Context, searchboxId string) *Error
}
