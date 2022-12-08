package reindexer

import (
	"context"
	"errors"

	"github.com/appbaseio/reactivesearch-api/model/reindex"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/prometheus/common/log"
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
	// Get the client ready for the request
	//
	// If the request is for a multi-tenant setup and the backend
	// is `system`, we need to use the system client to make the call.
	esClient, clientFetchErr := util.GetESClientForTenant(ctx)
	if clientFetchErr != nil {
		log.Warnln(logTag, ": ", clientFetchErr)
		return res, clientFetchErr
	}

	stats, err := esClient.IndexStats(indexName).Do(ctx)
	if err != nil {
		return res, err
	}

	if val, ok := stats.Indices[index]; ok {
		res = val.Primaries.Store.SizeInBytes
		return res, nil
	}

	return res, errors.New(`Invalid index name`)
}
