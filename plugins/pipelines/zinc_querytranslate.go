package pipelines

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	log "github.com/sirupsen/logrus"
)

// TranslateToZinc will translate the RS body to the equivalent
// zinc body that can be hit and the response can be retrieved.
//
// The function will return a stringified nd-JSON body that can be
// used directly to hit Zinc.
func TranslateToZinc(rsQuery *querytranslate.RSQuery, defaultIndex string) (string, *Error) {
	zincMSearchRequest := make([]map[string]interface{}, 0)

	for _, query := range rsQuery.Query {
		// If the query has `execute` as false, we need to skip that query.
		//
		// If `endpoint` is passed, it will be handled separately.
		if (query.Execute != nil && !*query.Execute) || ((query.EnableEndpointSuggestions == nil || *query.EnableEndpointSuggestions) && query.Endpoint != nil) {
			continue
		}

		zincedBody, translateErr := TranslateEachToZinc(&query, &rsQuery.Query, *rsQuery)
		if translateErr != nil {
			return "", translateErr
		}

		indexToUse := defaultIndex
		if query.Index != nil && *query.Index != "" {
			indexToUse = *query.Index
		}

		// Append the header first that will contain the index and then
		// the actual body.
		zincMSearchRequest = append(zincMSearchRequest, map[string]interface{}{"index": indexToUse})
		zincMSearchRequest = append(zincMSearchRequest, zincedBody)
	}

	// Marshal each map and join it using a \n to make it an nd-json
	marshalledBodyArr := make([]string, 0)
	for zincIndex, zincMapEach := range zincMSearchRequest {
		mapInBytes, marshalErr := json.Marshal(zincMapEach)
		if marshalErr != nil {
			return "", &Error{
				Err:  fmt.Errorf("error while marshalling zinc map at index: %d with err: %s", zincIndex, marshalErr),
				Code: http.StatusInternalServerError,
			}
		}

		marshalledBodyArr = append(marshalledBodyArr, string(mapInBytes))
	}

	return strings.Join(marshalledBodyArr, "\n"), nil
}

// TranslateEachToZinc will translate each query to its equivalent
// Zinc body.
func TranslateEachToZinc(query *querytranslate.Query, allQueries *[]querytranslate.Query, requestQuery querytranslate.RSQuery) (map[string]interface{}, *Error) {
	zincMap := make(map[string]interface{})

	if query.ID == nil || strings.Replace(*query.ID, " ", "", -1) == "" {
		return zincMap, &Error{
			Err:  fmt.Errorf("`id` is a required property for query and should be valid"),
			Code: http.StatusBadRequest,
		}
	}

	// Add a check to throw an error if type of query is `geo`.
	//
	// Since `geo` is not supported by Zinc yet, we need to throw an error if any
	// query is of type `geo`.
	if query.Type == querytranslate.Geo {
		errMsg := "`geo` is not supported when Zinc is backend"
		log.Warnln(logTag, ": ", errMsg)
		return zincMap, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// If type is range and either value or dataField is empty, we need to throw
	// an error.
	//
	// The error thrown here is that value and dataField will have to be passed
	// together if type is range and if neither is passed, that's okay as well.
	if query.Type == querytranslate.Range && query.Value != nil && query.DataField == nil {
		errMsg := "`dataField` is required when `value` is passed and type is `range`"
		return zincMap, &Error{
			Err:  fmt.Errorf(errMsg),
			Code: http.StatusBadRequest,
		}
	}

	// If `defaultQuery` is passed, we don't need to build the query since
	// we will use the user passed query
	isQueryPresent := false
	if query.DefaultQuery != nil {
		for key, value := range *query.DefaultQuery {
			// We'll copy query at the end if it is present
			if key == "query" {
				isQueryPresent = true
				continue
			}
			zincMap[key] = value
		}
	}

	// If `size` is not passed, set it as 10
	if query.Size == nil {
		defaultSize := 10
		query.Size = &defaultSize
	}
	sizeToUse := *query.Size
	if query.Type == querytranslate.Term {
		sizeToUse = 0
	}
	zincMap["size"] = sizeToUse

	// If `from` is passed, add it.
	if query.From != nil {
		zincMap["from"] = *query.From
	}

	// Add highlight support and highlightConfig support as well.
	query.ApplyHighlightQuery(&zincMap)

	// Support parsing the dataField value if it is passed.
	//
	// Since dataField is used in more than one place, we should
	// parse the normalizedFields and use it everywhere
	normalizedFields := make([]querytranslate.DataField, 0)
	if query.DataField != nil {
		normalizedFields = querytranslate.NormalizedDataFields(query.DataField, query.FieldWeights)
	}

	// If queryFormat is not passed, set it as or
	if query.QueryFormat == nil {
		defaultQF := "or"
		query.QueryFormat = &defaultQF
	}

	// Parse aggregations if the query type is term
	if len(normalizedFields) > 0 && query.Type == querytranslate.Term {
		termQuery := query.GetTermsAggsQuery(normalizedFields, 0)
		zincMap["aggs"] = termQuery
	}

	// Parse `react` if it's passed.
	if query.React != nil {
		var err error
		finalQuery := make([]interface{}, 0)
		finalOptions := make(map[string]interface{})
		finalQuery, err = querytranslate.EvalReactProp(finalQuery, &finalOptions, "", *query.React, requestQuery, generateZincQueryByType)
		if err != nil {
			errMsg := fmt.Sprintf("error while evaluating react prop: %s", err.Error())
			log.Warnln(logTag, ": ", errMsg)
			return finalOptions, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusInternalServerError,
			}
		}

		if len(finalQuery) != 0 {
			if query.DefaultQuery != nil {
				defaultQuery := *query.DefaultQuery
				if defaultQuery["query"] != nil {
					finalQuery = append(finalQuery, defaultQuery["query"])
				}
			} else if query.Type == querytranslate.Search || query.Type == querytranslate.Suggestion {
				// Only apply query by `value` for search queries
				queryByType, err := generateZincQueryByType(query)
				if err != nil {
					errMsg := fmt.Sprintf("error while generating query by type: %s", err.Error())
					log.Warnln(logTag, ": ", errMsg)
					return finalOptions, &Error{
						Err:  fmt.Errorf(errMsg),
						Code: http.StatusInternalServerError,
					}
				}
				if queryByType != nil && (queryByType == nil || reflect.ValueOf(queryByType).IsNil()) {
					finalQuery = append(finalQuery, queryByType)
				}
			}
			var boolQuery interface{} = map[string]interface{}{
				"bool": map[string]interface{}{
					"must": finalQuery,
				}}
			finalOptions["query"] = boolQuery

			// Apply the query properties like `highlight`, `from`, `size` etc.
			for key, value := range zincMap {
				finalOptions[key] = value
			}
			return finalOptions, nil
		}
	}

	if len(normalizedFields) > 0 && query.Value != nil && (query.Type == querytranslate.Search || query.Type == querytranslate.Suggestion) {
		zincMap["query"] = generateSearchQuery(normalizedFields, query)
	}

	// If `value` is not passed, we don't need to care about `dataField` in the
	// query so we can make it a match_all
	if query.Value == nil || query.Type == querytranslate.Term {
		zincMap["query"] = map[string]interface{}{"match_all": map[string]interface{}{}}
	}

	// Parse range query if the type is range
	if len(normalizedFields) > 0 && query.Type == querytranslate.Range {
		rangeQuery, rangeQueryErr := query.GenerateRangeQuery()
		if rangeQueryErr != nil {
			errMsg := fmt.Sprintf("error while building range query: %s", rangeQueryErr.Error())
			return zincMap, &Error{
				Err:  fmt.Errorf(errMsg),
				Code: http.StatusBadRequest,
			}
		}
		zincMap["query"] = rangeQuery
	}

	// Keeping `_source` empty will return all fields
	var sourceMap interface{}
	sourceMap = make([]string, 0)
	// Parse includeFields
	if query.IncludeFields != nil && len(*query.IncludeFields) > 0 {
		sourceMap = *query.IncludeFields
	}
	zincMap["_source"] = sourceMap

	// Parse the `dataField` to sort on. The first field of dataField will be
	// used for sorting.
	if query.SortBy != nil && len(normalizedFields) > 0 {
		// Parse the dataFields into normalized fields once again.
		sortField := normalizedFields[0].Field
		zincMap["sort"] = []map[string]interface{}{
			{
				sortField: map[string]interface{}{
					"order": query.SortBy.String(),
				},
			},
		}
	}

	// Add support for aggregations field as well
	if query.Aggregations != nil {
		if len(normalizedFields) < 1 {
			return zincMap, &Error{
				Err:  fmt.Errorf("field 'dataField' must be present to make 'aggregations' property work"),
				Code: http.StatusBadRequest,
			}
		}
		if query.Type == querytranslate.Range {
			zincMap = query.ApplyAggsForRange(normalizedFields, zincMap)
		}
	}

	if isQueryPresent {
		zincMap["query"] = (*query.DefaultQuery)["query"]
	}

	return zincMap, nil
}

// generateZincQueryByType will generate the zinc query based on
// the type se
func generateZincQueryByType(query *querytranslate.Query) (*interface{}, error) {
	var translatedQuery interface{}
	var translateError error
	switch query.Type {
	case querytranslate.Term:
		translatedQuery, translateError = query.GenerateTermQuery()
	case querytranslate.Range:
		translatedQuery, translateError = query.GenerateRangeQuery()
	case querytranslate.Suggestion, querytranslate.Search:
		normalizedFields := querytranslate.NormalizedDataFields(query.DataField, query.FieldWeights)
		translatedQuery, translateError = generateSearchQuery(normalizedFields, query), nil
	}
	return &translatedQuery, translateError
}

// generateSearchQuery will generate the `query` value for search
// and suggestion type of queries.
func generateSearchQuery(normalizedFields []querytranslate.DataField, query *querytranslate.Query) map[string]interface{} {
	shouldArr := make([]map[string]interface{}, 0)

	// If the value is nil then there's no use for the search query, so
	// we can directly return a match_all here.
	if query.Value == nil {
		return map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	if query.QueryFormat == nil {
		defaultQF := "or"
		query.QueryFormat = &defaultQF
	}

	// If length of `dataField` is 0, we should use a `query_string` match
	// with the passed value.
	if len(normalizedFields) < 1 {
		return map[string]interface{}{
			"query_string": map[string]interface{}{
				"query": *query.Value,
			},
		}
	}

	for _, normalizedField := range normalizedFields {
		queryMap := map[string]interface{}{
			"query":    *query.Value,
			"operator": *query.QueryFormat,
		}

		if normalizedField.Weight != 0 {
			queryMap["boost"] = normalizedField.Weight
		}

		shouldEachMap := map[string]interface{}{
			"match": map[string]interface{}{
				normalizedField.Field: queryMap,
			},
		}
		shouldArr = append(shouldArr, shouldEachMap)
	}
	return map[string]interface{}{"bool": map[string]interface{}{"should": shouldArr}}
}
