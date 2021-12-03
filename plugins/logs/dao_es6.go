package logs

import (
	"context"
	"encoding/json"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/util"
	es6 "gopkg.in/olivere/elastic.v6"
)

func (es *elasticsearch) getRawLogsES6(ctx context.Context, logsFilter logsFilter) ([]byte, error) {
	duration := es6.NewRangeQuery("timestamp").
		From(logsFilter.StartDate).
		To(logsFilter.EndDate)

	query := es6.NewBoolQuery().Filter(duration)
	// apply category filter
	if logsFilter.Filter == "search" {
		filters := es6.NewTermsQuery("category.keyword", []interface{}{"search", category.ReactiveSearch.String(), "suggestion"}...)
		query.Filter(filters)
	} else if logsFilter.Filter == "suggestion" {
		filters := es6.NewTermsQuery("category.keyword", []interface{}{"suggestion"}...)
		query.Filter(filters)
	} else if logsFilter.Filter == "index" {
		filters := []es6.Query{
			es6.NewTermsQuery("request.method.keyword", []interface{}{"POST", "PUT"}...),
			es6.NewTermsQuery("category.keyword", []interface{}{"docs"}...),
		}
		query.Filter(filters...)
	} else if logsFilter.Filter == "delete" {
		filters := es6.NewMatchQuery("request.method.keyword", "DELETE")
		query.Filter(filters)
	} else if logsFilter.Filter == "success" {
		filters := es6.NewRangeQuery("response.code").Gte(200).Lte(299)
		query.Filter(filters)
	} else if logsFilter.Filter == "error" {
		filters := es6.NewRangeQuery("response.code").Gte(400)
		query.Filter(filters)
	} else {
		query.Filter(es6.NewMatchAllQuery())
	}
	// apply index filtering logic
	util.GetIndexFilterQueryEs6(query, logsFilter.Filter)

	// only apply latency filter when start or end range is available
	if logsFilter.StartLatency != nil || logsFilter.EndLatency != nil {
		latencyRangeQuery := es6.NewRangeQuery("response.took")
		if logsFilter.StartLatency != nil {
			latencyRangeQuery.Gte(*logsFilter.StartLatency)
		}
		if logsFilter.EndLatency != nil {
			latencyRangeQuery.Lte(*logsFilter.EndLatency)
		}
		query.Filter(latencyRangeQuery)
	}

	searchQuery := util.GetClient6().Search(es.indexName).
		Query(query).
		From(logsFilter.Offset).
		Size(logsFilter.Size)
	if logsFilter.OrderByLatency != "" {
		ascending := false
		if logsFilter.OrderByLatency == "asc" {
			ascending = true
		}
		// sort by latency
		searchQuery.SortWithInfo(es6.SortInfo{Field: "response.took", UnmappedType: "int", Ascending: ascending})
	}
	searchQuery.SortWithInfo(es6.SortInfo{Field: "timestamp", UnmappedType: "date", Ascending: false})
	response, err := searchQuery.Do(ctx)

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
			log.Println(logTag, ": unable to find ", logsFilter.Indices, " in log record")
		}
		logIndices, err := util.ToStringSlice(rawIndices)
		if err != nil {
			log.Errorln(logTag, ":", err)
			continue
		}

		if len(logsFilter.Indices) == 0 {
			hits = append(hits, hit.Source)
		} else if util.IsSubset(logsFilter.Indices, logIndices) {
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
