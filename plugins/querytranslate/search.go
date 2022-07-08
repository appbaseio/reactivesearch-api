package querytranslate

import (
	"errors"
	"strconv"
	"strings"
)

// Generate the queryDSL for search type request
func (query *Query) generateSearchQuery() (*interface{}, error) {
	var searchQuery interface{}
	rankQuery := query.getRankFeatureQuery()

	if query.Value != nil {
		if query.QueryString != nil && *query.QueryString {
			shouldQuery, err := query.getSearchQuery()
			if err != nil {
				return nil, err
			}
			searchQuery = map[string]interface{}{
				"query_string": shouldQuery,
			}
		} else if query.SearchOperators != nil && *query.SearchOperators {
			shouldQuery, err := query.getSearchQuery()
			if err != nil {
				return nil, err
			}
			searchQuery = map[string]interface{}{
				"simple_query_string": shouldQuery,
			}
		} else {
			shouldQuery, err := query.getSearchQuery()
			if err != nil {
				return nil, err
			}
			minimumShouldMatch := 1
			// Use minimum_should_match value as 2 if rank query is present
			if rankQuery != nil {
				minimumShouldMatch = 2
			}
			searchQuery = map[string]interface{}{
				"bool": map[string]interface{}{
					"should":               shouldQuery,
					"minimum_should_match": minimumShouldMatch,
				},
			}
		}

		if query.CategoryValue != nil && query.CategoryField != nil &&
			*query.CategoryField != "" && *query.CategoryValue != "*" {
			searchQuery = map[string]interface{}{
				"bool": map[string]interface{}{
					"must": []interface{}{
						searchQuery.(map[string]interface{}),
						map[string]interface{}{
							"term": map[string]interface{}{
								*query.CategoryField: query.CategoryValue,
							},
						},
					},
				},
			}
		}
	} else if query.RankFeature != nil && rankQuery != nil {
		// Apply rank feature irrespective of value key
		searchQuery = map[string]interface{}{
			"bool": map[string]interface{}{
				"should":               &rankQuery,
				"minimum_should_match": 1,
			},
		}
		return &searchQuery, nil
	}

	if query.Value == nil {
		return nil, nil
	}
	// check if query value is empty string
	queryAsString, ok := (*query.Value).(string)
	if ok {
		if strings.TrimSpace(queryAsString) == "" {
			return nil, nil
		}
	}
	// check if query value is an empty array
	queryAsArray, ok := (*query.Value).([]interface{})
	if ok {
		if len(queryAsArray) == 0 {
			return nil, nil
		}
	}

	// Apply nestedField query
	searchQuery = query.applyNestedFieldQuery(searchQuery)

	return &searchQuery, nil
}

func (query *Query) getSearchQuery() (interface{}, error) {
	var queryValue []string

	// check if query value of string type
	queryAsString, ok := (*query.Value).(string)
	if ok {
		queryValue = []string{queryAsString}
	} else {
		// check if query value is array
		queryAsArray, ok := (*query.Value).([]interface{})
		if ok {
			for _, v := range queryAsArray {
				valueAsString, ok := v.(string)
				if ok {
					queryValue = append(queryValue, valueAsString)
				}
			}
		}
	}

	if len(queryValue) == 0 {
		return nil, nil
	}
	var isMultiSearch = false
	if len(queryValue) > 1 {
		isMultiSearch = true
	}
	// search query is a join of multiple values with space
	searchQuery := strings.Join(queryValue, " ")
	return query.generateShouldQueryByValue(searchQuery, isMultiSearch)
}

func (query *Query) generateShouldQueryByValue(value string, isMultiSearch bool) (interface{}, error) {
	var fields []string
	var phrasePrefixFields []string
	normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
	if len(normalizedFields) < 1 {
		return nil, errors.New("field 'dataField' cannot be empty")
	}
	for _, dataField := range normalizedFields {
		var fieldWeight string
		if dataField.Weight > 0 {
			fieldWeight = strconv.FormatFloat(dataField.Weight, 'f', 2, 64)
		}
		shouldIgnore := strings.HasSuffix(dataField.Field, ".keyword") ||
			strings.HasSuffix(dataField.Field, ".autosuggest") ||
			strings.HasSuffix(dataField.Field, ".search")
		if fieldWeight != "" {
			weightedField := dataField.Field + "^" + fieldWeight
			fields = append(fields, weightedField)
			if !shouldIgnore {
				// add fields for phrase_prefix with same weights normalized to 1.0
				// why reset weights: prefix query is meant to catch edge-cases that
				// are otherwise missed. The way it works is that it boosts the score
				// based on possible matches. This happens as a multiplication to the
				// weights set. Resetting the weights to 1 should reduce the boosting
				// factor of prefix queries.
				phrasePrefixFields = append(phrasePrefixFields, dataField.Field+"^1.0")
			}
		} else {
			fields = append(fields, dataField.Field)
			// add fields for phrase_prefix
			if !shouldIgnore {
				phrasePrefixFields = append(phrasePrefixFields, dataField.Field)
			}
		}
	}

	// Use default query format as or
	queryFormat := Or.String()
	if query.QueryFormat != nil {
		queryFormat = *query.QueryFormat
	}

	if query.QueryString != nil && *query.QueryString {
		return map[string]interface{}{
			"query":            value,
			"default_operator": queryFormat,
		}, nil
	}

	if query.SearchOperators != nil && *query.SearchOperators {
		return map[string]interface{}{
			"query":            value,
			"fields":           fields,
			"default_operator": queryFormat,
		}, nil
	}
	var valueAsInterface interface{} = value

	if queryFormat == And.String() {
		var finalQuery = []map[string]interface{}{
			{
				"multi_match": map[string]interface{}{
					"query":    getPlural(&valueAsInterface),
					"fields":   fields,
					"type":     "cross_fields",
					"operator": And.String(),
				},
			},
			{
				"multi_match": map[string]interface{}{
					"query":    value,
					"fields":   fields,
					"type":     "phrase",
					"operator": And.String(),
				},
			},
		}
		// Don't apply `phrase_prefix` query for multi-value search
		if len(phrasePrefixFields) > 0 && !isMultiSearch {
			finalQuery = append(finalQuery, map[string]interface{}{
				"multi_match": map[string]interface{}{
					"query":    value,
					"fields":   phrasePrefixFields,
					"type":     "phrase_prefix",
					"operator": And.String(),
				},
			})
		}
		if query.RankFeature != nil {
			for k, v := range *query.RankFeature {
				var rankFunction *FunctionObject
				var functionName string
				if v.Saturation != nil {
					rankFunction = v.Saturation
					functionName = "saturation"
				} else if v.Logarithm != nil {
					rankFunction = v.Logarithm
					functionName = "log"
				} else if v.Sigmoid != nil {
					rankFunction = v.Sigmoid
					functionName = "sigmoid"
				}
				if rankFunction != nil {
					rankFeatureQuery := map[string]interface{}{
						"field":      k,
						functionName: rankFunction,
					}
					if v.Boost != nil {
						rankFeatureQuery["boost"] = *v.Boost
					}
					finalQuery = append(finalQuery, map[string]interface{}{
						"rank_feature": rankFeatureQuery,
					})
				} else if v.Boost != nil {
					finalQuery = append(finalQuery, map[string]interface{}{
						"rank_feature": map[string]interface{}{
							"field": k,
							"boost": *v.Boost,
						},
					})
				}
			}
		}
		return finalQuery, nil
	}
	var fuzziness interface{}
	if query.Fuzziness != nil {
		fuzziness = query.Fuzziness
	} else {
		fuzziness = 0
	}

	var finalQuery = []map[string]interface{}{
		{
			"multi_match": map[string]interface{}{
				"query":    getPlural(&valueAsInterface),
				"fields":   fields,
				"type":     "cross_fields",
				"operator": Or.String(),
			},
		},
		{
			"multi_match": map[string]interface{}{
				"query":     valueAsInterface,
				"fields":    fields,
				"type":      "best_fields",
				"operator":  Or.String(),
				"fuzziness": fuzziness,
			},
		},
		{
			"multi_match": map[string]interface{}{
				"query":    valueAsInterface,
				"fields":   fields,
				"type":     "phrase",
				"operator": Or.String(),
			},
		},
	}

	rankQuery := query.getRankFeatureQuery()
	if rankQuery != nil {
		finalQuery = append(finalQuery, *rankQuery...)
	}

	if len(phrasePrefixFields) > 0 && !isMultiSearch {
		finalQuery = append(finalQuery, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":    valueAsInterface,
				"fields":   phrasePrefixFields,
				"type":     "phrase_prefix",
				"operator": Or.String(),
			},
		})
	}

	return finalQuery, nil
}

func (query *Query) getRankFeatureQuery() *[]map[string]interface{} {
	if query.RankFeature != nil {
		var rankQuery = make([]map[string]interface{}, 0)
		for k, v := range *query.RankFeature {
			var rankFunction *FunctionObject
			var functionName string
			if v.Saturation != nil {
				rankFunction = v.Saturation
				functionName = "saturation"
			} else if v.Logarithm != nil {
				rankFunction = v.Logarithm
				functionName = "log"
			} else if v.Sigmoid != nil {
				rankFunction = v.Sigmoid
				functionName = "sigmoid"
			}
			if rankFunction != nil {
				rankFeatureQuery := map[string]interface{}{
					"field":      k,
					functionName: rankFunction,
				}
				if v.Boost != nil {
					rankFeatureQuery["boost"] = *v.Boost
				}
				rankQuery = append(rankQuery, map[string]interface{}{
					"rank_feature": rankFeatureQuery,
				})
			} else if v.Boost != nil {
				rankQuery = append(rankQuery, map[string]interface{}{
					"rank_feature": map[string]interface{}{
						"field": k,
						"boost": *v.Boost,
					},
				})
			}
		}
		return &rankQuery
	}
	return nil
}
