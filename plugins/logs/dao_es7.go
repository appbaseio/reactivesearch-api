package logs

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
)

func (es *elasticsearch) getRawLogsES7(ctx context.Context, logsFilter logsFilter) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(logsFilter.StartDate).
		To(logsFilter.EndDate)

	query := es7.NewBoolQuery().Filter(duration)
	// apply category filter
	if logsFilter.Filter == "search" {
		filters := es7.NewTermsQuery("category.keyword", []interface{}{"search", category.ReactiveSearch.String()}...)
		query.Filter(filters)
	} else if logsFilter.Filter == "delete" {
		filters := es7.NewMatchQuery("request.method.keyword", "DELETE")
		query.Filter(filters)
	} else if logsFilter.Filter == "success" {
		filters := es7.NewRangeQuery("response.code").Gte(200).Lte(299)
		query.Filter(filters)
	} else if logsFilter.Filter == "error" {
		filters := es7.NewRangeQuery("response.code").Gte(400)
		query.Filter(filters)
	} else {
		query.Filter(es7.NewMatchAllQuery())
	}

	// apply index filtering logic
	util.GetIndexFilterQueryEs7(query, logsFilter.Indices...)

	response, err := util.GetClient7().Search(es.indexName).
		Query(query).
		From(logsFilter.Offset).
		Size(logsFilter.Size).
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
