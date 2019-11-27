package logs

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticSearch) getRawLogsES7(ctx context.Context, from string, size int, filter string, offset int, indices ...string) ([]byte, error) {
	query := es7.NewBoolQuery()
	// apply category filter
	if filter == "search" {
		filters := es7.NewTermQuery("category.keyword", "search")
		query.Filter(filters)
	} else if filter == "delete" {
		filters := es7.NewMatchQuery("request.method.keyword", "DELETE")
		query.Filter(filters)
	} else if filter == "success" {
		filters := es7.NewRangeQuery("response.code").Gte(200).Lte(299)
		query.Filter(filters)
	} else if filter == "error" {
		filters := es7.NewRangeQuery("response.code").Gte(400)
		query.Filter(filters)
	} else {
		query.Filter(es7.NewMatchAllQuery())
	}

	// apply index filtering logic
	util.GetIndexFilterQueryEs7(query, indices...)

	response, err := util.GetClient7().Search(es.indexName).
		Query(query).
		From(offset).
		Size(size).
		SortWithInfo(es7.SortInfo{Field: "timestamp", UnmappedType: "date", Ascending: false}).
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
