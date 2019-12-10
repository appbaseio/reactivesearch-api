package logs

import (
	"context"
	"encoding/json"
	"log"

	"github.com/appbaseio/arc/util"
	es6 "gopkg.in/olivere/elastic.v6"
)

func (es *elasticsearch) getRawLogsES6(ctx context.Context, from string, size int, filter string, offset int, indices ...string) ([]byte, error) {
	query := es6.NewBoolQuery()
	// apply category filter
	if filter == "search" {
		filters := es6.NewTermQuery("category.keyword", "search")
		query.Filter(filters)
	} else if filter == "delete" {
		filters := es6.NewMatchQuery("request.method.keyword", "DELETE")
		query.Filter(filters)
	} else if filter == "success" {
		filters := es6.NewRangeQuery("response.code").Gte(200).Lte(299)
		query.Filter(filters)
	} else if filter == "error" {
		filters := es6.NewRangeQuery("response.code").Gte(400)
		query.Filter(filters)
	} else {
		query.Filter(es6.NewMatchAllQuery())
	}
	// apply index filtering logic
	util.GetIndexFilterQueryEs6(query, indices...)

	response, err := util.GetClient6().Search(es.indexName).
		Query(query).
		From(offset).
		Size(size).
		SortWithInfo(es6.SortInfo{Field: "timestamp", UnmappedType: "date", Ascending: false}).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := []*json.RawMessage{}
	for _, hit := range response.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(*hit.Source, &source)
		if err != nil {
			return nil, err
		}
		rawIndices, ok := source["indices"]
		if !ok {
			log.Printf(`%s: unable to find "indices" in log record\n`, logTag)
		}
		logIndices, err := util.ToStringSlice(rawIndices)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			continue
		}

		if len(indices) == 0 {
			hits = append(hits, hit.Source)
		} else if util.IsSubset(indices, logIndices) {
			hits = append(hits, hit.Source)
		}
	}

	logs := make(map[string]interface{})
	logs["logs"] = hits
	logs["total"] = len(hits)
	logs["took"] = response.TookInMillis

	raw, err := json.Marshal(logs)
	if err != nil {
		return nil, err
	}

	return raw, nil
}
