package querytranslate

import "errors"

func (query *Query) generateTermQuery() (*interface{}, error) {

	if query.Value == nil {
		return nil, nil
	}

	var termQuery interface{}
	value := *query.Value
	valueAsArray, isArray := value.([]interface{})
	valueAsString := value
	if (isArray && len(valueAsArray) == 0) || (valueAsString == "") {
		return nil, nil
	}

	normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
	if len(normalizedFields) < 1 {
		return nil, errors.New("field 'dataField' cannot be empty")
	}
	dataField := normalizedFields[0].Field

	if query.SelectAllLabel != nil && (isArray && contains(valueAsArray, *query.SelectAllLabel) || valueAsString == *query.SelectAllLabel) {
		if query.ShowMissing != nil && *query.ShowMissing {
			termQuery = map[string]interface{}{
				"match_all": map[string]interface{}{},
			}
		} else {
			termQuery = map[string]interface{}{
				"exists": map[string]interface{}{
					"field": dataField,
				},
			}
		}
		return &termQuery, nil
	}

	if len(valueAsArray) != 0 {
		// Use default query format as or
		queryFormat := Or.String()
		if query.QueryFormat != nil {
			queryFormat = *query.QueryFormat
		}
		queryType := "term"
		if queryFormat == Or.String() {
			queryType = "terms"
		}
		if queryFormat == Or.String() {
			var should = []map[string]interface{}{
				{
					queryType: map[string]interface{}{
						dataField: query.filterValue(valueAsArray),
					},
				},
			}
			if query.ShowMissing != nil && *query.ShowMissing {
				hasMissingTerm := contains(valueAsArray, query.MissingLabel)
				if hasMissingTerm {
					should = append(should, map[string]interface{}{
						"bool": map[string]interface{}{
							"must_not": map[string]interface{}{
								"exists": map[string]interface{}{
									"field": dataField,
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
						dataField: item,
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
		if query.ShowMissing != nil && *query.ShowMissing && query.MissingLabel == valueAsString {
			termQuery = map[string]interface{}{
				"bool": map[string]interface{}{
					"must_not": map[string]interface{}{
						"exists": map[string]interface{}{
							"field": dataField,
						},
					},
				},
			}
		} else {
			termQuery = map[string]interface{}{
				"term": map[string]interface{}{
					dataField: valueAsString,
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

		if query.ShowMissing != nil && *query.ShowMissing {
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

func (query *Query) applyTermsAggsQuery(queryOptions *map[string]interface{}) error {
	if queryOptions != nil {
		normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
		if len(normalizedFields) < 1 {
			return errors.New("field 'dataField' cannot be empty")
		}
		dataField := normalizedFields[0].Field
		clonedQuery := *queryOptions

		termsQuery := map[string]interface{}{
			"field": dataField,
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
				"_key": &query.SortBy,
			}
		}

		// Apply missing label
		if query.ShowMissing != nil && *query.ShowMissing {
			if query.MissingLabel != "" {
				termsQuery["missing"] = query.MissingLabel
			} else {
				termsQuery["missing"] = "N/A"
			}
		}

		termQuery := map[string]interface{}{
			dataField: map[string]interface{}{
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
	return nil
}
