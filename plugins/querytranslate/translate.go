package querytranslate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// transform the query
func translateQuery(rsQuery RSQuery, userIP string, queryForId *string) (string, []byte, error) {
	// Validate custom events
	if rsQuery.Settings != nil && rsQuery.Settings.CustomEvents != nil {
		for k, v := range *rsQuery.Settings.CustomEvents {
			_, ok := v.(string)
			if !ok {
				valueAsInterface, ok := v.([]interface{})
				if !ok {
					return "", nil, errors.New("Custom event " + k + " value must be a string or an array of strings")
				}
				for _, v1 := range valueAsInterface {
					_, ok := v1.(string)
					if !ok {
						return "", nil, errors.New("Custom event " + k + " value must be a string or an array of strings")
					}
				}
			}
		}
	}
	var mSearchQuery string
	for queryIndex, query := range rsQuery.Query {
		// Validate ID
		if query.ID == nil {
			return "", nil, errors.New("field 'id' can't be empty")
		}
		normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)

		// Validate multiple DataFields for term and geo queries
		if (query.Type == Geo) && len(normalizedFields) > 1 {
			return "", nil, errors.New("field 'dataField' can not have multiple fields for 'geo' query")
		}

		// Validate highlight and highlightConfig
		if query.HighlightConfig != nil && (query.Highlight == nil || !*query.Highlight) {
			return "", nil, errors.New("`highlightConfig` will be ignored when `highlight` is not passed or set to `false`")
		}

		// Normalize query value for search and suggestion types of queries
		if query.Type == Suggestion {
			if query.Value != nil {
				// set the updated value
				var err error
				rsQuery.Query[queryIndex].Value, err = normalizeQueryValue(query.Value)
				if err != nil {
					return "", nil, err
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
			}
		}

		// Validate the endpoint property
		if query.Endpoint != nil {
			if query.Endpoint.URL == nil || *query.Endpoint.URL == "" {
				return "", nil, errors.New("`endpoint.url` is a required property when `endpoint` is passed. Remove the `endpoint` property if it's not used.")
			}

			// Setting the default method etc will be done during
			// sending the independent queries and not in this part of the code.
		}

	}

	// If no backend is passed for kNN, set it as `elasticsearch`
	backendPassed := ElasticSearch
	if rsQuery.Settings != nil && rsQuery.Settings.Backend != nil {
		backendPassed = *rsQuery.Settings.Backend
	}

	for _, query := range rsQuery.Query {

		// If the endpoint property is passed, set the query execute as false
		if query.Endpoint != nil {
			if query.EnableEndpointSuggestions == nil || *query.EnableEndpointSuggestions {
				executeValue := false
				query.Execute = &executeValue
			}
		}

		if query.shouldExecuteQuery() {
			translatedQuery, queryOptions, isGeneratedByValue, translateError := query.getQuery(rsQuery)
			if translateError != nil {
				return mSearchQuery, nil, translateError
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
				return mSearchQuery, nil, err
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
				return mSearchQuery, nil, err2
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
				return mSearchQuery, nil, err
			}
			// Build final query
			mSearchQuery += string(preferenceInBytes)
			mSearchQuery += "\n"
			mSearchQuery += string(queryInBytes)
			mSearchQuery += "\n"
			if queryForId != nil {
				return mSearchQuery, queryInBytes, nil
			}
		}
	}

	return mSearchQuery, nil, nil
}

// buildIndependentRequests will build the requests that have the endpoint
// property passed and will accordingly generate an array of objects
// that will be hit one by one during searching.
func buildIndependentRequests(rsQuery RSQuery) ([]map[string]interface{}, error) {
	independentQueryArr := make([]map[string]interface{}, 0)

	for _, query := range rsQuery.Query {

		if query.Endpoint == nil {
			continue
		} else if query.EnableEndpointSuggestions != nil && !*query.EnableEndpointSuggestions {
			continue
		}

		queryAsMap, queryBuildErr := BuildIndependentRequest(query, rsQuery)
		if queryBuildErr != nil {
			return independentQueryArr, queryBuildErr
		}

		independentQueryArr = append(independentQueryArr, queryAsMap)
	}

	return independentQueryArr, nil
}

// Just a wrapper around buildIndependentRequests to let it be accessible
// outside the plugin
func BuildIndependentRequests(rsQuery RSQuery) ([]map[string]interface{}, error) {
	return buildIndependentRequests(rsQuery)
}

// BuildIndependentRequest will build the independent request based on the passed
// details and return a map to be used during execution of the request.
func BuildIndependentRequest(query Query, rsQuery RSQuery) (map[string]interface{}, error) {
	DEFAULT_METHOD := http.MethodGet
	DEFAULT_HEADERS := make(map[string]string)

	if query.Endpoint.Method == nil || *query.Endpoint.Method == "" {
		// Set to default endpoint
		query.Endpoint.Method = &DEFAULT_METHOD
	}

	// If headers are not passed, set it as empty headers
	if query.Endpoint.Headers == nil {
		query.Endpoint.Headers = &DEFAULT_HEADERS
	}

	// If body is not passed, pass the current body without
	// the endpoint property.
	if query.Endpoint.Body == nil && *query.Endpoint.Method == http.MethodPost {
		// Generate the body without the endpoint part
		queryAsMap := make(map[string]interface{})

		queryAsBytes, marshalErr := json.Marshal(query)
		if marshalErr != nil {
			log.Warnln(logTag, ": error while marshalling body without the endpoint property, ", marshalErr)
			return nil, marshalErr
		}

		unmarshalErr := json.Unmarshal(queryAsBytes, &queryAsMap)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("error while unmarshalling body without endpoint property into a map, ", unmarshalErr)
			log.Warnln(logTag, ": ", errMsg)
			return nil, fmt.Errorf(errMsg)
		}

		delete(queryAsMap, "endpoint")

		bodyToSend := map[string]interface{}{
			"query": []map[string]interface{}{
				queryAsMap,
			},
		}

		if rsQuery.Settings != nil {
			bodyToSend["settings"] = *rsQuery.Settings
		}
		if rsQuery.Metadata != nil {
			bodyToSend["metadata"] = *rsQuery.Metadata
		}

		query.Endpoint.Body = new(interface{})
		*query.Endpoint.Body = bodyToSend
	}

	builtQuery, marshalErr := json.Marshal(*query.Endpoint)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshalling query for hitting independently, ", marshalErr)
		return nil, marshalErr
	}

	// Unmarshal the built query into a map
	queryAsMap := make(map[string]interface{})
	endpointAsMap := make(map[string]interface{})

	unmarshalErr := json.Unmarshal(builtQuery, &endpointAsMap)
	if unmarshalErr != nil {
		log.Warnln(logTag, ": error while unmarshalling query for hitting independently, ", unmarshalErr)
		return nil, unmarshalErr
	}

	queryAsMap["id"] = *query.ID
	queryAsMap["endpoint"] = endpointAsMap

	return queryAsMap, nil
}

// RemoveEndpointRecursionIfRS will remove the endpoint response's recursion if
// the response is of RS structure.
//
// RS response will be considered if the response if of type map and the only
// key inside the response is the ID of the query.
func RemoveEndpointRecursionIfRS(resp []byte, queryID string) ([]byte, error) {
	responseMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(resp, &responseMap)

	if unmarshalErr != nil {
		// Since the response is not of type map, can't be an
		// RS response.
		return resp, nil
	}

	// Check if the only ID present in the map is the queryID and `settings`.
	isRSResponse := true

	for key := range responseMap {
		log.Debug(logTag, ": key: ", key)
		if key != queryID && key != "settings" {
			isRSResponse = false
			break
		}
	}

	if !isRSResponse {
		return resp, nil
	}

	responseToReturn := responseMap[queryID]
	return json.Marshal(responseToReturn)
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
func TranslateQuery(rsQuery RSQuery, userIP string, queryForId *string) (string, []byte, error) {
	return translateQuery(rsQuery, userIP, queryForId)
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
	case Suggestion:
		translatedQuery, translateError = query.generateSuggestionQuery()
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
	//
	// Following will only be reached if the sortField is passed
	// or sortBy is passed. This also means that the following criterion
	// will make sure that sorting on `_score` is done only if neither of
	// them are passed and in that case we don't pass the sort key at all.
	//
	// Above explanation indicates that we can set the sortBy value to `ascending`
	// if it is not passed without checking whether the sortField is `_score` because
	// when the sortField is score, it will not go in the following block.
	if (query.SortBy != nil || query.SortField != nil) && query.Type == Search {
		// If both sortField and dataFields are not present
		// then raise an error.
		if len(normalizedFields) < 1 && query.SortField == nil {
			return nil, errors.New("field 'dataField' or `sortField` must be present to apply 'sortBy' property")
		}

		// If sortBy is nil, set it to Desc
		if query.SortBy == nil {
			defaultSortBy := Asc
			query.SortBy = &defaultSortBy
		}

		// sortField can be a string, an array of strings or an array of objects
		// and strings
		// where the key indicates the field to sort on and the value is
		// one of valid sort types.
		//
		// For string or array of strings, the value of `sortBy` will be
		// considered.

		sortFieldParsed := make(map[string]SortBy)

		// If not passed, just set the dataField as the sortField with
		// the value of sortBy.
		if query.SortField == nil {
			dataField := normalizedFields[0].Field
			sortFieldParsed[dataField] = *query.SortBy
		} else {
			// Parse the sortField accordingly.
			var sortFieldParseErr error
			sortFieldParsed, sortFieldParseErr = ParseSortField(*query, *query.SortBy)
			if sortFieldParseErr != nil {
				return nil, sortFieldParseErr
			}
		}

		// Change the following to support proper formatting of sortField
		sortValue := make([]map[string]interface{}, 0)
		for sortField, sortBy := range sortFieldParsed {
			sortValue = append(sortValue, map[string]interface{}{
				sortField: map[string]interface{}{
					"order": sortBy,
				},
			})
		}

		queryWithOptions["sort"] = sortValue
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

		if query.IncludeValues != nil {
			termsQuery["include"] = *query.IncludeValues
		}
		if query.ExcludeValues != nil {
			termsQuery["exclude"] = *query.ExcludeValues
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
			queryWithOptions = query.ApplyAggsForRange(normalizedFields, queryWithOptions)
		}
	}
	return queryWithOptions, nil
}

// ApplyAggsForRange will build the aggregations for range type of
// query.
//
// The function will inject aggs related fields to the query and return
// the update map.
func (query *Query) ApplyAggsForRange(normalizedFields []DataField, queryWithOptions map[string]interface{}) map[string]interface{} {
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

	return queryWithOptions
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

// ApplyHighlightQuery is a wrapper to apply highlight query into the passed
// query map
func (query *Query) ApplyHighlightQuery(queryOptions *map[string]interface{}) {
	query.applyHighlightQuery(queryOptions)
}
