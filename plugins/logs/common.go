package logs

import (
	es7 "github.com/olivere/elastic/v7"
	es6 "gopkg.in/olivere/elastic.v6"
)

// GetFilterQueryEs6 filters the logs by category
func GetFilterQueryEs6(query *es6.BoolQuery, filter string) *es6.BoolQuery {
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
	return query
}

// GetFilterQueryEs7 filters the logs by category
func GetFilterQueryEs7(query *es7.BoolQuery, filter string) *es7.BoolQuery {
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
	return query
}
