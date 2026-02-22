package suggestions

import (
	"context"

	es7 "github.com/olivere/elastic/v7"
)

type suggestionService interface {
	setAlias(ctx context.Context, originalIndex, timeStampedIndex string) (interface{}, error)
	populateTimeStampedIndex(ctx context.Context, timestampedIndex string) (interface{}, error)
}

type suggestionMetaService interface {
	savePopularSuggestionsPreferences(ctx context.Context, record PopularPreferences) (*es7.IndexResponse, error)
	saveIndexSuggestionsPreferences(ctx context.Context, record IndexPreferences) (*es7.IndexResponse, error)
	saveRecentSuggestionsPreferences(ctx context.Context, record RecentPreferences) (*es7.IndexResponse, error)
	updateLastSyncTime(ctx context.Context) (interface{}, error)
	getPopularSuggestionsPreferences(ctx context.Context) (PopularPreferences, error)
	getIndexSuggestionsPreferences(ctx context.Context) (IndexPreferences, error)
	getRecentSuggestionsPreferences(ctx context.Context) (RecentPreferences, error)
}
