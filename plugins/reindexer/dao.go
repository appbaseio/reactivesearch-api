package reindexer

import (
	"context"
	"errors"

	"github.com/appbaseio/reactivesearch-api/model/reindex"
	"github.com/appbaseio/reactivesearch-api/util"
)

func getIndexSize(ctx context.Context, indexName string) (int64, error) {
	var res int64
	index := indexName
	aliasesIndexMap, err := reindex.GetAliasIndexMap(ctx)
	if err != nil {
		return res, err
	}
	if indexNameFromMap, ok := aliasesIndexMap[indexName]; ok {
		index = indexNameFromMap
	}

	stats, err := util.GetClient7().IndexStats(indexName).Do(ctx)
	if err != nil {
		return res, err
	}

	if val, ok := stats.Indices[index]; ok {
		res = val.Primaries.Store.SizeInBytes
		return res, nil
	}

	return res, errors.New(`Invalid index name`)
}
