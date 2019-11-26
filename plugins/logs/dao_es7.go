package logs

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *ElasticSearch) getRawLogsES7(ctx context.Context, from string, size int, filter string, offset int, indices ...string) ([]byte, error) {
	query := es7.NewBoolQuery()

	// apply category filter
	GetFilterQueryEs7(query, filter)

	// apply index filtering logic
	util.GetIndexFilterQueryEs7(query, indices...)

	response, err := es.client7.Search(es.indexName).
		Query(query).
		From(offset).
		Size(size).
		Sort("timestamp", false).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := []json.RawMessage{}
	for _, hit := range response.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(hit.Source, &source)
		if err != nil {
			return nil, err
		}
		hits = append(hits, hit.Source)
	}

	logs := make(map[string]interface{})
	logs["logs"] = hits
	logs["total"] = response.Hits.TotalHits.Value
	logs["took"] = response.TookInMillis

	raw, err := json.Marshal(logs)
	if err != nil {
		return nil, err
	}
	return raw, nil
}
