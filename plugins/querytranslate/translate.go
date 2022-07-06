package querytranslate

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// transform the query
func translateQuery(rsQuery RSQuery, userIP string) (string, error) {
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
			return "", errors.New("field 'id' can't be empty")
		}
		normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)

		// Validate multiple DataFields for term and geo queries
		if (query.Type == Term || query.Type == Geo) && len(normalizedFields) > 1 {
			return "", errors.New("field 'dataField' can not have multiple fields for 'term' or 'geo' queries")
		}

		// Validate highlight and highlightConfig
		if query.HighlightConfig != nil && (query.Highlight == nil || !*query.Highlight) {
			return "", errors.New("`highlightConfig` will be ignored when `highlight` is not passed or set to `false`")
		}

		// Normalize query value for search and suggestion types of queries
		if query.Type == Search || query.Type == Suggestion {
			if query.Value != nil {
				// set the updated value
				var err error
				rsQuery.Query[queryIndex].Value, err = normalizeQueryValue(query.Value)
				if err != nil {
					return "", err
				}
			}
		}

		// Parse synonyms fields if `EnableSynonyms` is set to `false`
		if (query.Type == Search || query.Type == Suggestion) && query.EnableSynonyms != nil && !*query.EnableSynonyms {
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
				return "", errors.New("you're using .synonyms suffix fields in the 'dataField' property but 'enableSynonyms' property is set to `false`. We recommend removing these fields from the Search Settings UI / API or set enableSynonyms to true")
			}
		}

		// Validate the endpoint property
		if query.Endpoint != nil {
			if query.Endpoint.URL == nil || *query.Endpoint.URL == "" {
				return "", errors.New("`endpoint.url` is a required property when `endpoint` is passed. Remove the `endpoint` property if it's not used.")
			}

			DEFAULT_METHOD := http.MethodGet

			if query.Endpoint.Method == nil || *query.Endpoint.Method == "" {
				// Set to default endpoint
				query.Endpoint.Method = &DEFAULT_METHOD
			}
		}

	}

	// If no backend is passed for kNN, set it as `elasticsearch`
	backendPassed := ElasticSearch
	if rsQuery.Settings != nil && rsQuery.Settings.Backend != nil {
		backendPassed = *rsQuery.Settings.Backend
	}

	for _, query := range rsQuery.Query {
		if query.shouldExecuteQuery() {
			translatedQuery, queryOptions, isGeneratedByValue, translateError := query.getQuery(rsQuery)
			if translateError != nil {
				return mSearchQuery, translateError
			}
			// Set match_all query if query is nil or query is `term` but generated by value property
			if isNilInterface(*translatedQuery) || (query.Type == Term && isGeneratedByValue) {
				var matchAllQuery interface{} = map[string]interface{}{
					"match_all": map[string]interface{}{},
				}
				translatedQuery = &matchAllQuery
			}
			// Set query options coming from react prop
			finalQuery := queryOptions
			finalQuery["query"] = translatedQuery

			// Handle DeepPagination
			// NOTE: Following code should be before `from` is added to the final
			// query because deepPagination might modify the from value.
			if query.DeepPagination == nil {
				defaultDeepPagination := false
				query.DeepPagination = &defaultDeepPagination
			}

			// If deep pagination is enabled, set it to search_after
			// since this translation is happening for ES.
			if *query.DeepPagination &&
				query.DeepPaginationConfig != nil &&
				query.DeepPaginationConfig.Cursor != nil &&
				*query.DeepPaginationConfig.Cursor != "" {
				// Set the from value of the request to 0
				fromForSearchAfter := 0
				query.From = &fromForSearchAfter

				// Add the search_after field.
				searchAfterValue := []string{*query.DeepPaginationConfig.Cursor}
				finalQuery["search_after"] = searchAfterValue
			}

			// Apply query options
			buildQueryOptions, err := query.buildQueryOptions()
			if err != nil {
				return mSearchQuery, err
			}
			finalQuery = mergeMaps(finalQuery, buildQueryOptions)
			// Apply defaultQuery if present
			if query.DefaultQuery != nil {
				defaultQueryClone := make(map[string]interface{})
				// Apply default query without query key
				for k, v := range *query.DefaultQuery {
					if k != "query" {
						defaultQueryClone[k] = v
					}
				}
				finalQuery = mergeMaps(finalQuery, defaultQueryClone)
			}

			// If knn fields are passed, apply knn fields to the final query
			if shouldApplyKnn(query) {
				// Apply default candidate number if nothing is passed
				if query.Candidates == nil {
					defaultCandidates := 10
					query.Candidates = &defaultCandidates
				}

				if query.Size == nil {
					defaultSize := 10
					query.Size = &defaultSize
				}

				minSize := *query.Candidates
				if minSize > *query.Size {
					minSize = *query.Size
				}

				// Set default script for the backend if none
				// is passed
				if query.Script == nil {
					defaultScript := GetDefaultScript(backendPassed)
					query.Script = &defaultScript
				}

				switch backendPassed {
				case ElasticSearch:
					finalQuery = applyElasticSearchKnn(finalQuery, query, minSize)
				case OpenSearch:
					finalQuery = applyOpenSearchKnn(finalQuery, query, minSize)
				}
			}

			queryInBytes, err2 := json.Marshal(finalQuery)
			if err2 != nil {
				return mSearchQuery, err2
			}
			// Add preference
			preferenceId := *query.ID + "_" + userIP
			if rsQuery.Settings != nil && rsQuery.Settings.UserID != nil {
				preferenceId = *query.ID + "_" + *rsQuery.Settings.UserID
			}
			var msearchConfig = map[string]interface{}{
				"preference": preferenceId,
			}
			if query.Index != nil {
				msearchConfig["index"] = *query.Index
			}
			preferenceInBytes, err := json.Marshal(msearchConfig)
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

// shouldApplyKnn determines whether or not to apply KNN stage
func shouldApplyKnn(query Query) bool {
	return query.QueryVector != nil && query.VectorDataField != nil
}

// applyElasticSearchKnn applies the knn query for elasticsearch
// backend
func applyElasticSearchKnn(queryMap map[string]interface{}, queryItem Query, size int) map[string]interface{} {
	// Replace the query field
	currentQuery := queryMap["query"]
	updatedQuery := map[string]interface{}{
		"script_score": map[string]interface{}{
			"query": currentQuery,
			"script": map[string]interface{}{
				"source": *queryItem.Script,
				"params": map[string]interface{}{
					"queryVector": *queryItem.QueryVector,
					"dataField":   *queryItem.VectorDataField,
				},
			},
		},
	}

	// Update the queryMap
	queryMap["query"] = updatedQuery

	// Set the size
	queryMap["size"] = size

	return queryMap
}

// applyOpenSearchKnn applies the knn query for opensearch backend
//
// The structure is just a bit different to how it's applied for ES
func applyOpenSearchKnn(queryMap map[string]interface{}, queryItem Query, size int) map[string]interface{} {
	// Replace the query field
	currentQuery := queryMap["query"]
	updatedQuery := map[string]interface{}{
		"script_score": map[string]interface{}{
			"query": currentQuery,
			"script": map[string]interface{}{
				"source": "knn_score",
				"lang":   "knn",
				"params": map[string]interface{}{
					"query_value": *queryItem.QueryVector,
					"field":       *queryItem.VectorDataField,
					"space_type":  *queryItem.Script,
				},
			},
		},
	}

	// Update the queryMap
	queryMap["query"] = updatedQuery

	// Set the size
	queryMap["size"] = size

	return queryMap
}

// GetDefaultScript returns the default script for the passed backend
func GetDefaultScript(backend Backend) string {
	switch backend {
	case ElasticSearch:
		return "cosineSimilarity(params.queryVector, params.dataField) + 1.0"
	case OpenSearch:
		return "cosinesimil"
	}

	return ""
}

// global function to transform the RS API query to _msearch equivalent query
func TranslateQuery(rsQuery RSQuery, userIP string) (string, error) {
	return translateQuery(rsQuery, userIP)
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
	if (query.SortBy != nil || query.SortField != nil) && query.Type == Search {
		// If both sortField and dataFields are not present
		// then raise an error.
		if len(normalizedFields) < 1 && query.SortField == nil {
			return nil, errors.New("field 'dataField' or `sortField` must be present to apply 'sortBy' property")
		}

		// sortField get's priority
		// if not present and normalized field is present
		// then it is assigned.
		if query.SortField == nil {
			dataField := normalizedFields[0].Field
			query.SortField = &dataField
		}

		// If sortBy is nil, set it to Desc
		if query.SortBy == nil {
			defaultSortBy := Desc
			query.SortBy = &defaultSortBy
		}

		queryWithOptions["sort"] = []map[string]interface{}{
			{
				*query.SortField: map[string]interface{}{
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
				return nil, errors.New("field 'dataField' must be present to make 'pagination' work for 'term' type of queries")
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
	if query.CategoryField != nil &&
		*query.CategoryField != "" &&
		(query.Type == Search || query.Type == Suggestion) {
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
			return nil, errors.New("field 'dataField' must be present to make 'aggregations' property work")
		}
		if query.Type == Range {
			dataField := normalizedFields[0].Field
			tempAggs := *query.Aggregations
			rangeAggs := map[string]interface{}{}
			if util.Contains(tempAggs, "min") {
				rangeAggs["min"] = map[string]interface{}{
					"min": map[string]interface{}{"field": dataField}}
			}
			if util.Contains(tempAggs, "max") {
				rangeAggs["max"] = map[string]interface{}{
					"max": map[string]interface{}{"field": dataField}}
			}

			if util.Contains(tempAggs, "histogram") {
				if query.CalendarInterval != nil {
					// run date histogram query
					rangeAggs[dataField] = map[string]interface{}{
						"date_histogram": map[string]interface{}{
							"field":             dataField,
							"calendar_interval": *query.CalendarInterval,
						},
					}
				} else {
					// rangeHistogram can work without range value as well
					// so it being nil should not have an effect.

					// If range value is not present, just create a dummy one.
					var dummyStartEndValue interface{} = 0
					rangeValue := &RangeValue{
						Start: &dummyStartEndValue,
						End:   &dummyStartEndValue,
					}

					var err error

					useStartValue := false

					if query.Value != nil {
						rangeValue, err = query.getRangeValue(*query.Value)
						useStartValue = true
					}

					if err != nil {
						log.Errorln(logTag, ":", err)
					} else if rangeValue != nil && rangeValue.Start != nil && rangeValue.End != nil {
						histogramMap := map[string]interface{}{
							"field":    dataField,
							"interval": getValidInterval(query.Interval, *rangeValue),
						}

						if useStartValue {
							histogramMap["offset"] = rangeValue.Start
						}

						rangeAggs[dataField] = map[string]interface{}{
							"histogram": histogramMap,
						}
					}
				}
			}

			if util.Contains(tempAggs, "date-histogram") && query.Value != nil {
				rangeValue, err := query.getRangeValue(*query.Value)
				if err != nil {
					log.Errorln(logTag, ":", err)
				} else if rangeValue != nil && rangeValue.Start != nil && rangeValue.End != nil {
					rangeAggs[dataField] = map[string]interface{}{
						"histogram": map[string]interface{}{
							"field":    dataField,
							"interval": getValidInterval(query.Interval, *rangeValue),
							"offset":   rangeValue.Start,
						},
					}
				}
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
		}
	}
	return queryWithOptions, nil
}

func (query *Query) applyNestedFieldQuery(originalQuery interface{}) interface{} {
	if !isNilInterface(originalQuery) && query.NestedField != nil {
		nestedFieldQuery :=
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
		if query.HighlightConfig != nil || query.CustomHighlight != nil {
			var highlightConfig map[string]interface{}
			if query.HighlightConfig != nil {
				highlightConfig = *query.HighlightConfig
			} else if query.CustomHighlight != nil {
				highlightConfig = *query.CustomHighlight
			}
			if highlightConfig["fields"] == nil {
				fields := make(map[string]interface{})
				var highlightFields = query.HighlightField
				if len(highlightFields) == 0 {
					// use data fields as highlighted field
					dataFields := NormalizedDataFields(query.DataField, []float64{})
					for _, v := range dataFields {
						highlightFields = append(highlightFields, v.Field)
					}
				}
				for _, field := range highlightFields {
					fields[field] = make(map[string]interface{})
				}
				highlightConfig["fields"] = fields
			}
			clonedQuery["highlight"] = highlightConfig
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
