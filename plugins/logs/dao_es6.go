package logs

import (
	"context"
	"encoding/json"
	"log"

	"github.com/appbaseio/arc/util"
	es6 "gopkg.in/olivere/elastic.v6"
)

func (es *ElasticSearch) getRawLogsES6(ctx context.Context, from string, size int, filter string, offset int, indices ...string) ([]byte, error) {
	query := es6.NewBoolQuery()
	// apply category filter
	GetFilterQueryEs6(query, filter)
	// apply index filtering logic
	util.GetIndexFilterQueryEs6(query, indices...)

	response, err := es.client6.Search(es.indexName).
		Query(query).
		From(offset).
		Size(size).
		Sort("timestamp", false).
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
