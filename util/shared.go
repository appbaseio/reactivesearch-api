package util

import (
	"context"

	es7 "github.com/olivere/elastic/v7"
	es6 "gopkg.in/olivere/elastic.v6"
)

// GetIndexFilterQueryEs6 apply the index filtering logic
func GetIndexFilterQueryEs6(query *es6.BoolQuery, indices ...string) *es6.BoolQuery {
	if indices != nil && len(indices) > 0 {
		var indexQueries []es6.Query
		for _, index := range indices {
			query := es6.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Should(indexQueries...)
	}
	return query
}

// GetIndexFilterQueryEs7 apply the index filtering logic
func GetIndexFilterQueryEs7(query *es7.BoolQuery, indices ...string) *es7.BoolQuery {
	if indices != nil && len(indices) > 0 {
		var indexQueries []es7.Query
		for _, index := range indices {
			query := es7.NewTermQuery("indices.keyword", index)
			indexQueries = append(indexQueries, query)
		}
		query = query.Should(indexQueries...)
	}
	return query
}

// GetTotalNodes retrieves the number of es nodes
func GetTotalNodes() (int, error) {
	response, err := GetClient7().NodesInfo().
		Metric("nodes").
		Do(context.Background())
	if err != nil {
		return -1, err
	}
	return len(response.Nodes), nil
}
