package reindexer

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/model/reindex"
)

func getIndexSize(ctx context.Context, indexName string) (int64, error) {
	return reindex.GetIndexStoreSize(ctx, indexName)
}
