package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	"github.com/prometheus/common/log"
)

func (es *elasticsearch) getRawLogsES7(ctx context.Context, logsFilter logsFilter) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(logsFilter.StartDate).
		To(logsFilter.EndDate)

	query := es7.NewBoolQuery().Filter(duration)
	// apply category filter
	if logsFilter.Filter == "search" {
		filters := es7.NewTermsQuery("category.keyword", []interface{}{"search", category.ReactiveSearch.String(), "suggestion"}...)
		query.Filter(filters)
	} else if logsFilter.Filter == "suggestion" {
		filters := es7.NewTermsQuery("category.keyword", []interface{}{"suggestion"}...)
		query.Filter(filters)
	} else if logsFilter.Filter == "index" {
		filters := []es7.Query{
			es7.NewTermsQuery("request.method.keyword", []interface{}{"POST", "PUT"}...),
			es7.NewTermsQuery("category.keyword", []interface{}{"docs"}...),
			es7.NewRangeQuery("response.code").Gte(200).Lte(299),
		}
		query.Filter(filters...)
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

	// only apply latency filter when start or end range is available
	if logsFilter.StartLatency != nil || logsFilter.EndLatency != nil {
		latencyRangeQuery := es7.NewRangeQuery("response.took")
		if logsFilter.StartLatency != nil {
			latencyRangeQuery.Gte(*logsFilter.StartLatency)
		}
		if logsFilter.EndLatency != nil {
			latencyRangeQuery.Lte(*logsFilter.EndLatency)
		}
		query.Filter(latencyRangeQuery)
	}

	searchQuery := util.GetClient7().Search(es.indexName).
		Query(query).
		From(logsFilter.Offset).
		Size(logsFilter.Size)
	if logsFilter.OrderByLatency != "" {
		ascending := false
		if logsFilter.OrderByLatency == "asc" {
			ascending = true
		}
		// sort by latency
		searchQuery.SortWithInfo(es7.SortInfo{Field: "response.took", UnmappedType: "int", Ascending: ascending})
	}
	searchQuery.SortWithInfo(es7.SortInfo{Field: "timestamp", UnmappedType: "date", Ascending: false})
	response, err := searchQuery.Do(ctx)
	if err != nil {
		return nil, err
	}

	hits := make([]map[string]interface{}, 0)
	for _, hit := range response.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(hit.Source, &source)
		if err != nil {
			return nil, err
		}

		// Extract the log ID
		source["id"] = hit.Id

		hits = append(hits, source)
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

// getRawLogES7 will get the raw log for the log with passed ID.
// If we don't find a match, we will raise a 404 error.
func (es *elasticsearch) getRawLogES7(ctx context.Context, ID string) ([]byte, *LogError) {
	query := es7.NewTermQuery("_id", ID)

	response, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error while running query on ID")
	}

	if len(response.Hits.Hits) == 0 {
		return nil, &LogError{
			Error: errors.New(fmt.Sprintf("Log not found with ID: %s", ID)),
			Code:  http.StatusNotFound,
		}
	}

	log := make(map[string]interface{})
	logMatched := response.Hits.Hits[0]

	err = json.Unmarshal(logMatched.Source, &log)
	if err != nil {
		return nil, &LogError{
			Error: errors.New("Error occurred while unmarshalling log hit"),
			Code:  http.StatusInternalServerError,
		}
	}

	// Add the ID
	log["id"] = logMatched.Id

	// Marshal and return
	rawLog, err := json.Marshal(log)
	if err != nil {
		return nil, &LogError{
			Error: errors.New("error occurred while marshalling log body"),
			Code:  http.StatusInternalServerError,
		}
	}

	return rawLog, nil

}

type LogError struct {
	Error error
	Code  int
}
