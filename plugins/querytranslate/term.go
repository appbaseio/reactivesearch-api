package querytranslate

func (query *Query) generateTermQuery(rsQuery RSQuery) (*interface{}, error) {

	if query.Value == nil {
		return nil, nil
	}

	var termQuery interface{}
	value := *query.Value
	valueAsArray, isArray := value.([]interface{})
	valueAsString, isString := value.(interface{})
	if (isArray && len(valueAsArray) == 0) || (isString && valueAsString == "") {
		return nil, nil
	}

	if query.SelectAllLabel != nil && (isArray && contains(valueAsArray, *query.SelectAllLabel) || isString && valueAsString == *query.SelectAllLabel) {
		if query.ShowMissing {
			termQuery = map[string]interface{}{
				"match_all": map[string]interface{}{},
			}
		} else {
			termQuery = map[string]interface{}{
				"exists": map[string]interface{}{
					"field": query.DataField[0],
				},
			}
		}
		return &termQuery, nil
	}

	if len(valueAsArray) != 0 {
		// Use default query format as or
		queryFormat := Or
		if query.QueryFormat != nil {
			queryFormat = *query.QueryFormat
		}
		queryType := "term"
		if queryFormat.String() == Or.String() {
			queryType = "terms"
		}
		if queryFormat.String() == Or.String() {
			var should = []map[string]interface{}{
				{
					queryType: map[string]interface{}{
						query.DataField[0]: query.filterValue(valueAsArray),
					},
				},
			}
			if query.ShowMissing {
				hasMissingTerm := contains(valueAsArray, query.MissingLabel)
				if hasMissingTerm {
					should = append(should, map[string]interface{}{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": query.DataField[0],
								},
							},
						},
					})
				}
			}
			termQuery = &map[string]interface{}{
				"bool": map[string]interface{}{
					"should": should,
				},
			}
		} else {
			// adds a sub-query with must as an array of objects for each term/value
			var queryArray []map[string]interface{}
			for _, item := range valueAsArray {
				queryArray = append(queryArray, map[string]interface{}{
					queryType: map[string]interface{}{
						query.DataField[0]: item,
					},
				})
			}
			termQuery = &map[string]interface{}{
				"bool": map[string]interface{}{
					"must": queryArray,
				},
			}
		}
	} else if valueAsString != "" {
		// Handle value as string, for singleList components
		if query.ShowMissing && query.MissingLabel == valueAsString {
			termQuery = map[string]interface{}{
				"bool": map[string]interface{}{
					"must_not": map[string]interface{}{
						"exists": map[string]interface{}{
							"field": query.DataField[0],
						},
					},
				},
			}
		} else {
			termQuery = map[string]interface{}{
				"term": map[string]interface{}{
					query.DataField[0]: valueAsString,
				},
			}
		}
	}

	// Apply nestedField query
	termQuery = query.applyNestedFieldQuery(termQuery)

	return &termQuery, nil
}

func (query *Query) filterValue(ss []interface{}) (ret []interface{}) {
	for _, item := range ss {
		if item != query.MissingLabel {
			ret = append(ret, item)
		}
	}
	return
}

func (query *Query) applyCompositeAggsQuery(queryOptions *map[string]interface{}, aggsField string) {
	if queryOptions != nil && aggsField != "" {
		clonedQuery := *queryOptions

		termsQuery := map[string]interface{}{
			"field": aggsField,
		}

		if query.ShowMissing {
			termsQuery["missing_bucket"] = true
		}

		// Note: composite aggs only allows asc and desc order
		if query.SortBy != nil && *query.SortBy != Count {
			termsQuery["order"] = &query.SortBy
		}

		compositeQuery := map[string]interface{}{
			"sources": []map[string]interface{}{
				{
					aggsField: map[string]interface{}{
						"terms": termsQuery,
					},
				},
			},
		}

		if query.After != nil {
			compositeQuery["after"] = query.After
		}

		if query.AggregationSize != nil {
			compositeQuery["size"] = query.AggregationSize
		} else if query.Size != nil {
			compositeQuery["size"] = query.Size
		}

		fieldQuery := map[string]interface{}{
			"composite": compositeQuery,
		}

		// apply top hits query if aggregationField is defined
		if query.AggregationField != nil {
			fieldQuery["aggs"] = map[string]interface{}{
				aggsField: map[string]interface{}{
					"top_hits": map[string]interface{}{
						"size": 1,
					},
				},
			}
		}

		aggsQuery := map[string]interface{}{
			aggsField: fieldQuery,
		}
		clonedQuery["aggs"] = aggsQuery
	}
}

func (query *Query) applyTermsAggsQuery(queryOptions *map[string]interface{}) {
	if queryOptions != nil {
		clonedQuery := *queryOptions

		termsQuery := map[string]interface{}{
			"field": query.DataField[0],
		}

		if query.AggregationSize != nil {
			termsQuery["size"] = query.AggregationSize
		} else if query.Size != nil {
			termsQuery["size"] = query.Size
		}

		// Apply sortBy, defaults to `count`
		if query.SortBy == nil || *query.SortBy == Count {
			termsQuery["order"] = map[string]interface{}{
				"_count": "desc",
			}
		} else {
			termsQuery["order"] = map[string]interface{}{
				"_term": &query.SortBy,
			}
		}

		// Apply missing label
		if query.ShowMissing {
			if query.MissingLabel != "" {
				termsQuery["missing"] = query.MissingLabel
			} else {
				termsQuery["missing"] = "N/A"
			}
		}

		termQuery := map[string]interface{}{
			query.DataField[0]: map[string]interface{}{
				"terms": termsQuery,
			},
		}

		if query.NestedField != nil {
			clonedQuery["aggs"] = map[string]interface{}{
				"reactivesearch_nested": map[string]interface{}{
					"nested": map[string]interface{}{
						"path": *query.NestedField,
					},
					"aggs": termQuery,
				},
			}
		} else {
			clonedQuery["aggs"] = termQuery
		}
	}
}
