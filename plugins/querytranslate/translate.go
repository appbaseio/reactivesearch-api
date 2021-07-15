package querytranslate

import (
	"encoding/json"
	"errors"
	"strings"

	log "github.com/sirupsen/logrus"
)

// transform the query
func translateQuery(rsQuery RSQuery) (string, error) {
	// Validate custom events
	if rsQuery.Settings != nil && rsQuery.Settings.CustomEvents != nil {
		for k, v := range *rsQuery.Settings.CustomEvents {
			_, ok := v.(string)
			if !ok {
				valueAsInterface, ok := v.([]interface{})
				if !ok {
					return "", errors.New("Custom event " + k + " value must be a string or an array of strings")
				}
				for _, v1 := range valueAsInterface {
					_, ok := v1.(string)
					if !ok {
						return "", errors.New("Custom event " + k + " value must be a string or an array of strings")
					}
				}
			}
		}
	}
	var mSearchQuery string
	for queryIndex, query := range rsQuery.Query {
		// Validate ID
		if query.ID == nil {
			return "", errors.New("Field 'id' can't be empty")
		}
		normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)

		// Validate multiple DataFields for term and geo queries
		if (query.Type == Term || query.Type == Geo) && len(normalizedFields) > 1 {
			return "", errors.New("Field 'dataField' can not have multiple fields for 'term' or 'geo' queries")
		}

		// Parse synonyms fields if `EnableSynonyms` is set to `false`
		if query.Type == Search && query.EnableSynonyms != nil && !*query.EnableSynonyms {
			var normalizedDataFields = []string{}
			for _, dataField := range normalizedFields {
				if !strings.HasSuffix(dataField.Field, synonymsFieldKey) {
					normalizedDataFields = append(normalizedDataFields, dataField.Field)
				}
			}
			if len(normalizedDataFields) > 0 {
				// Set the updated fields
				rsQuery.Query[queryIndex].DataField = normalizedDataFields
			} else {
				return "", errors.New("You're using .synonyms suffix in all the fields defined in the 'dataField' property but 'enableSynonyms' property is set to `false` which is contradictory")
			}
		}
	}

	for _, query := range rsQuery.Query {
		if query.Execute == nil || *query.Execute {
			translatedQuery, queryOptions, isGeneratedByReact, translateError := query.getQuery(rsQuery)
			if translateError != nil {
				return mSearchQuery, translateError
			}
			// Set match_all query if query is nil or query is `term` but not generated by react dependencies
			if isNilInterface(*translatedQuery) || (query.Type == Term && !isGeneratedByReact) {
				var matchAllQuery interface{}
				matchAllQuery = map[string]interface{}{
					"match_all": map[string]interface{}{},
				}
				translatedQuery = &matchAllQuery
			}
			// Set query options coming from react prop
			finalQuery := queryOptions
			finalQuery["query"] = translatedQuery

			// Apply query options
			buildQueryOptions, err := query.buildQueryOptions()
			if err != nil {
				return mSearchQuery, err
			}
			finalQuery = mergeMaps(finalQuery, buildQueryOptions)
			// Apply defaultQuery if present
			if query.DefaultQuery != nil {
				finalQuery = mergeMaps(finalQuery, *query.DefaultQuery)
			}
			queryInBytes, err2 := json.Marshal(finalQuery)
			if err2 != nil {
				return mSearchQuery, err2
			}
			// Add preference
			var preference = map[string]interface{}{
				"preference": query.ID,
			}
			if query.Index != nil {
				preference["index"] = *query.Index
			}
			preferenceInBytes, err := json.Marshal(preference)
			if err != nil {
				return mSearchQuery, err
			}
			// Build final query
			mSearchQuery += string(preferenceInBytes)
			mSearchQuery += "\n"
			mSearchQuery += string(queryInBytes)
			mSearchQuery += "\n"
		}
	}

	return mSearchQuery, nil
}

// Generate the queryDSL without options for a particular query type
func (query *Query) generateQueryByType() (*interface{}, error) {
	var translatedQuery interface{}
	var translateError error
	switch query.Type {
	case Term:
		translatedQuery, translateError = query.generateTermQuery()
	case Range:
		translatedQuery, translateError = query.generateRangeQuery()
	case Geo:
		translatedQuery, translateError = query.generateGeoQuery()
	default:
		translatedQuery, translateError = query.generateSearchQuery()
	}
	return &translatedQuery, translateError
}

// Builds the query options for e.g `size`, `from`, `highlight` etc.
func (query *Query) buildQueryOptions() (map[string]interface{}, error) {
	queryWithOptions := make(map[string]interface{})
	if query.Size != nil {
		queryWithOptions["size"] = query.Size
	} else if query.Type == Term || query.Type == Range {
		// Set default size value as `zero`
		queryWithOptions["size"] = 0
	}

	// Apply `distinctField` and `distinctFieldConfig` props
	if query.DistinctFieldConfig != nil {
		collapseConfig := *query.DistinctFieldConfig
		if query.DistinctField != nil {
			collapseConfig["field"] = *query.DistinctField
		}
		queryWithOptions["collapse"] = collapseConfig
	} else if query.DistinctField != nil {
		queryWithOptions["collapse"] = map[string]interface{}{
			"field": *query.DistinctField,
		}
	}

	// Don't apply from for `term` type
	if query.From != nil && query.Type != Term {
		queryWithOptions["from"] = query.From
	}

	normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)

	// Only apply sort on search queries
	if query.SortBy != nil && query.Type == Search {
		if len(normalizedFields) < 1 {
			return nil, errors.New("Field 'dataField' must be present to apply 'sortBy' property")
		}
		dataField := normalizedFields[0].Field
		queryWithOptions["sort"] = []map[string]interface{}{
			{
				dataField: map[string]interface{}{
					"order": *query.SortBy,
				},
			},
		}
	}

	includeFields := []string{"*"}
	excludeFields := []string{}
	if query.IncludeFields != nil {
		includeFields = *query.IncludeFields
	}
	if query.ExcludeFields != nil {
		excludeFields = *query.ExcludeFields
	}
	queryWithOptions["_source"] = map[string]interface{}{
		"includes": includeFields,
		"excludes": excludeFields,
	}

	// Apply highlight query
	query.applyHighlightQuery(&queryWithOptions)
	/**
	Note: `aggregationField` doesn't work with list components
	*/
	if query.Type == Term {
		// If pagination is true then use composite aggregations
		if query.Pagination != nil && *query.Pagination {
			if len(normalizedFields) < 1 {
				return nil, errors.New("Field 'dataField' must be present to make 'pagination' work for 'term' type of queries")
			}
			dataField := normalizedFields[0].Field
			query.applyCompositeAggsQuery(&queryWithOptions, dataField)
		} else {
			query.applyTermsAggsQuery(&queryWithOptions)
		}
	} else if query.AggregationField != nil {
		query.applyCompositeAggsQuery(&queryWithOptions, *query.AggregationField)
	}

	// Apply category aggs
	if query.CategoryField != nil && query.Type == Search {
		// Add aggregations for the category
		aggs := make(map[string]interface{})
		termsQuery := map[string]interface{}{
			"field": query.CategoryField,
		}
		// apply size for categories
		if query.AggregationSize != nil {
			termsQuery["size"] = query.AggregationSize
		}
		aggs[*query.CategoryField] = map[string]interface{}{
			"terms": termsQuery,
		}
		// Merge with the aggs added by aggregation field
		aggsAddedByAggField, isAggExists := queryWithOptions["aggs"].(map[string]interface{})
		if isAggExists {
			queryWithOptions["aggs"] = mergeMaps(aggs, aggsAddedByAggField)
		} else {
			queryWithOptions["aggs"] = aggs
		}
	}

	// Apply aggs from aggregations field
	if query.Aggregations != nil {
		if len(normalizedFields) < 1 {
			return nil, errors.New("Field 'dataField' must be present to make 'aggregations' property work")
		}
		if query.Type == Range {
			dataField := normalizedFields[0].Field
			tempAggs := *query.Aggregations
			if len(tempAggs) == 2 && tempAggs[0] == "min" && tempAggs[1] == "max" {
				rangeAggs := map[string]interface{}{
					"min": map[string]interface{}{
						"min": map[string]interface{}{"field": dataField}},
					"max": map[string]interface{}{
						"max": map[string]interface{}{"field": dataField}},
				}
				if query.NestedField != nil {
					tempNestedField := *query.NestedField
					queryWithOptions["aggs"] = map[string]interface{}{
						tempNestedField: map[string]interface{}{
							"nested": map[string]interface{}{
								"path": tempNestedField,
							},
							"aggs": rangeAggs,
						},
					}
				} else {
					queryWithOptions["aggs"] = rangeAggs
				}

			} else if len(tempAggs) == 1 && tempAggs[0] == "histogram" && query.Value != nil {

				rangeValue, err := query.getRangeValue(*query.Value)
				if err != nil {
					log.Errorln(logTag, ":", err)
				} else if rangeValue != nil && rangeValue.Start != nil && rangeValue.End != nil {
					queryWithOptions["aggs"] = map[string]interface{}{
						dataField: map[string]interface{}{
							"histogram": map[string]interface{}{
								"field":    dataField,
								"interval": getValidInterval(query.Interval, *rangeValue),
								"offset":   rangeValue.Start,
							},
						},
					}
				}
			}
		}
	}
	return queryWithOptions, nil
}

func (query *Query) applyNestedFieldQuery(originalQuery interface{}) interface{} {
	if !isNilInterface(originalQuery) && query.NestedField != nil {
		var nestedFieldQuery interface{}
		nestedFieldQuery =
			map[string]interface{}{
				"nested": map[string]interface{}{
					"path":  query.NestedField,
					"query": originalQuery,
				},
			}
		return &nestedFieldQuery
	}
	return originalQuery
}

// Adds the highlight query
func (query *Query) applyHighlightQuery(queryOptions *map[string]interface{}) {
	clonedQuery := *queryOptions
	if query.Highlight != nil && *query.Highlight {
		if query.CustomHighlight != nil {
			clonedQuery["highlight"] = *query.CustomHighlight
		} else {
			var fields = make(map[string]interface{})
			var highlightField = query.HighlightField
			normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
			if len(highlightField) == 0 {
				for _, v := range normalizedFields {
					highlightField = append(highlightField, v.Field)
				}
			}
			for _, field := range highlightField {
				fields[field] = make(map[string]interface{})
			}

			highlightOptions := map[string]interface{}{
				"pre_tags":  []string{"<mark>"},
				"post_tags": []string{"</mark>"},
				"fields":    fields,
			}
			if len(query.HighlightField) != 0 {
				highlightOptions["require_field_match"] = false
			}
			clonedQuery["highlight"] = highlightOptions
		}
	}
}