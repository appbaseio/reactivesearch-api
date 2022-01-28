package querytranslate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/bbalet/stopwords"
	pluralize "github.com/gertd/go-pluralize"
	"github.com/kljensen/snowball"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/microcosm-cc/bluemonday"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var RESERVED_KEYS_IN_RESPONSE = []string{"settings", "error"}

// EXCEPTION_KEYS_IN_QUERY represents the keys which will not get copied while combining the queries using `react` prop
var EXCEPTION_KEYS_IN_QUERY = []string{"size", "from", "aggs", "_source", "query"}

type FunctionObject struct {
	// works with Saturation
	Pivot *float64 `json:"pivot,omitempty"`
	// Pivot and Exponent work with Sigmoid
	Exponent *float64 `json:"exponent,omitempty"`
	// works with Logarithm
	ScalingFactor *float64 `json:"scaling_factor,omitempty"`
}

type RankFunction struct {
	Saturation *FunctionObject `json:"saturation,omitempty"`
	Logarithm  *FunctionObject `json:"log,omitempty"`
	Sigmoid    *FunctionObject `json:"sigmoid,omitempty"`
	Boost      *float64        `json:"boost,omitempty"`
}

type QueryType int

const synonymsFieldKey = ".synonyms"

const (
	Search QueryType = iota
	Term
	Range
	Geo
	Suggestion
)

// String is the implementation of Stringer interface that returns the string representation of QueryType type.
func (o QueryType) String() string {
	return [...]string{
		"search",
		"term",
		"range",
		"geo",
		"suggestion",
	}[o]
}

type SuggestionType int

const (
	Index SuggestionType = iota
	Popular
	Recent
	Promoted
)

// String is the implementation of Stringer interface that returns the string representation of SuggestionType type.
func (o SuggestionType) String() string {
	return [...]string{
		"index",
		"popular",
		"recent",
		"promoted",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling SuggestionType type.
func (o *SuggestionType) UnmarshalJSON(bytes []byte) error {
	var suggestionType string
	err := json.Unmarshal(bytes, &suggestionType)
	if err != nil {
		return err
	}
	switch suggestionType {
	case Index.String():
		*o = Index
	case Popular.String():
		*o = Popular
	case Recent.String():
		*o = Recent
	case Promoted.String():
		*o = Promoted
	default:
		return fmt.Errorf("invalid suggestion type encountered: %v", suggestionType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling SuggestionType type.
func (o SuggestionType) MarshalJSON() ([]byte, error) {
	var suggestionType string
	switch o {
	case Index:
		suggestionType = Index.String()
	case Popular:
		suggestionType = Popular.String()
	case Recent:
		suggestionType = Recent.String()
	case Promoted:
		suggestionType = Promoted.String()
	default:
		return nil, fmt.Errorf("invalid suggestion type encountered: %v", o)
	}
	return json.Marshal(suggestionType)
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling QueryType type.
func (o *QueryType) UnmarshalJSON(bytes []byte) error {
	var queryType string
	err := json.Unmarshal(bytes, &queryType)
	if err != nil {
		return err
	}
	switch queryType {
	case Search.String():
		*o = Search
	case Term.String():
		*o = Term
	case Range.String():
		*o = Range
	case Geo.String():
		*o = Geo
	case Suggestion.String():
		*o = Suggestion
	default:
		return fmt.Errorf("invalid queryType encountered: %v", queryType)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling QueryType type.
func (o QueryType) MarshalJSON() ([]byte, error) {
	var queryType string
	switch o {
	case Search:
		queryType = Search.String()
	case Term:
		queryType = Term.String()
	case Range:
		queryType = Range.String()
	case Geo:
		queryType = Geo.String()
	case Suggestion:
		queryType = Suggestion.String()
	default:
		return nil, fmt.Errorf("invalid queryType encountered: %v", o)
	}
	return json.Marshal(queryType)
}

type SortBy int

const (
	Asc SortBy = iota
	Desc
	Count
)

// String is the implementation of Stringer interface that returns the string representation of SortBy type.
func (o SortBy) String() string {
	return [...]string{
		"asc",
		"desc",
		"count",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling SortBy type.
func (o *SortBy) UnmarshalJSON(bytes []byte) error {
	var sortBy string
	err := json.Unmarshal(bytes, &sortBy)
	if err != nil {
		return err
	}
	switch sortBy {
	case Asc.String():
		*o = Asc
	case Desc.String():
		*o = Desc
	case Count.String():
		*o = Count
	default:
		return fmt.Errorf("invalid sortBy encountered: %v", sortBy)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling SortBy type.
func (o SortBy) MarshalJSON() ([]byte, error) {
	var sortBy string
	switch o {
	case Asc:
		sortBy = Asc.String()
	case Desc:
		sortBy = Desc.String()
	case Count:
		sortBy = Count.String()
	default:
		return nil, fmt.Errorf("invalid sortBy encountered: %v", o)
	}
	return json.Marshal(sortBy)
}

type QueryFormat int

const (
	Or QueryFormat = iota
	And
)

// String is the implementation of Stringer interface that returns the string representation of QueryFormat type.
func (o QueryFormat) String() string {
	return [...]string{
		"or",
		"and",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling QueryFormat type.
func (o *QueryFormat) UnmarshalJSON(bytes []byte) error {
	var queryFormat string
	err := json.Unmarshal(bytes, &queryFormat)
	if err != nil {
		return err
	}
	switch queryFormat {
	case Or.String():
		*o = Or
	case And.String():
		*o = And
	default:
		return fmt.Errorf("invalid queryFormat encountered: %v", queryFormat)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling QueryFormat type.
func (o QueryFormat) MarshalJSON() ([]byte, error) {
	var queryFormat string
	switch o {
	case Or:
		queryFormat = Or.String()
	case And:
		queryFormat = And.String()
	default:
		return nil, fmt.Errorf("invalid queryFormat encountered: %v", o)
	}
	return json.Marshal(queryFormat)
}

// Query represents the query object
type Query struct {
	ID                          *string                    `json:"id,omitempty"` // component id
	Type                        QueryType                  `json:"type,omitempty"`
	React                       *map[string]interface{}    `json:"react,omitempty"`
	QueryFormat                 *string                    `json:"queryFormat,omitempty"`
	DataField                   interface{}                `json:"dataField,omitempty"`
	CategoryField               *string                    `json:"categoryField,omitempty"`
	CategoryValue               *interface{}               `json:"categoryValue,omitempty"`
	FieldWeights                []float64                  `json:"fieldWeights,omitempty"`
	NestedField                 *string                    `json:"nestedField,omitempty"`
	From                        *int                       `json:"from,omitempty"`
	Size                        *int                       `json:"size,omitempty"`
	AggregationSize             *int                       `json:"aggregationSize,omitempty"`
	SortBy                      *SortBy                    `json:"sortBy,omitempty"`
	Value                       *interface{}               `json:"value,omitempty"` // either string or Array of string
	AggregationField            *string                    `json:"aggregationField,omitempty"`
	After                       *map[string]interface{}    `json:"after,omitempty"`
	IncludeNullValues           *bool                      `json:"includeNullValues,omitempty"`
	IncludeFields               *[]string                  `json:"includeFields,omitempty"`
	ExcludeFields               *[]string                  `json:"excludeFields,omitempty"`
	Fuzziness                   interface{}                `json:"fuzziness,omitempty"` // string or int
	SearchOperators             *bool                      `json:"searchOperators,omitempty"`
	Highlight                   *bool                      `json:"highlight,omitempty"`
	HighlightField              []string                   `json:"highlightField,omitempty"`
	CustomHighlight             *map[string]interface{}    `json:"customHighlight,omitempty"`
	HighlightConfig             *map[string]interface{}    `json:"highlightConfig,omitempty"`
	Interval                    *int                       `json:"interval,omitempty"`
	Aggregations                *[]string                  `json:"aggregations,omitempty"`
	MissingLabel                string                     `json:"missingLabel,omitempty"`
	ShowMissing                 bool                       `json:"showMissing,omitempty"`
	DefaultQuery                *map[string]interface{}    `json:"defaultQuery,omitempty"`
	CustomQuery                 *map[string]interface{}    `json:"customQuery,omitempty"`
	Execute                     *bool                      `json:"execute,omitempty"`
	EnableSynonyms              *bool                      `json:"enableSynonyms,omitempty"`
	SelectAllLabel              *string                    `json:"selectAllLabel,omitempty"`
	Pagination                  *bool                      `json:"pagination,omitempty"`
	QueryString                 *bool                      `json:"queryString,omitempty"`
	RankFeature                 *map[string]RankFunction   `json:"rankFeature,omitempty"`
	DistinctField               *string                    `json:"distinctField,omitempty"`
	DistinctFieldConfig         *map[string]interface{}    `json:"distinctFieldConfig,omitempty"`
	Index                       *string                    `json:"index,omitempty"`
	EnableRecentSuggestions     *bool                      `json:"enableRecentSuggestions,omitempty"`
	RecentSuggestionsConfig     *RecentSuggestionsOptions  `json:"recentSuggestionsConfig,omitempty"`
	EnablePopularSuggestions    *bool                      `json:"enablePopularSuggestions,omitempty"`
	PopularSuggestionsConfig    *PopularSuggestionsOptions `json:"popularSuggestionsConfig,omitempty"`
	ShowDistinctSuggestions     *bool                      `json:"showDistinctSuggestions,omitempty"`
	EnablePredictiveSuggestions *bool                      `json:"enablePredictiveSuggestions,omitempty"`
	MaxPredictedWords           *int                       `json:"maxPredictedWords,omitempty"`
	URLField                    *string                    `json:"urlField,omitempty"`
	ApplyStopwords              *bool                      `json:"applyStopwords,omitempty"`
	Stopwords                   *[]string                  `json:"customStopwords,omitempty"`
	SearchLanguage              *string                    `json:"searchLanguage,omitempty"`
	CalendarInterval            *string                    `json:"calendarinterval,omitempty"`
}

type DataField struct {
	Field  string  `json:"field"`
	Weight float64 `json:"weight,omitempty"`
}

// Settings represents the search settings
type Settings struct {
	RecordAnalytics       *bool                   `json:"recordAnalytics,omitempty"`
	UserID                *string                 `json:"userId,omitempty"`
	CustomEvents          *map[string]interface{} `json:"customEvents,omitempty"`
	EnableQueryRules      *bool                   `json:"enableQueryRules,omitempty"`
	EnableSearchRelevancy *bool                   `json:"enableSearchRelevancy,omitempty"`
	UseCache              *bool                   `json:"useCache,omitempty"`
	QueryRule             *map[string]interface{} `json:"queryRule,omitempty"`
}

// RSQuery represents the request body
type RSQuery struct {
	Query    []Query   `json:"query,omitempty"`
	Settings *Settings `json:"settings,omitempty"`
}

type TermFilter struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// QueryEnvs represents the extracted values from RSQuery
type QueryEnvs struct {
	Query       *string
	TermFilters []TermFilter
}

// ExtractEnvsFromRequest returns the extracted values from RS request
func ExtractEnvsFromRequest(req RSQuery) QueryEnvs {
	var queryEnvs = QueryEnvs{}
	var termFilters []TermFilter
	for _, query := range req.Query {
		// Set query
		if (query.Type == Search || query.Type == Suggestion) && query.Value != nil {
			value := *query.Value
			valueAsString, ok := value.(string)
			if ok {
				// Use query in lower case
				queryLowerCase := strings.ToLower(valueAsString)
				queryEnvs.Query = &queryLowerCase
			}
		}
		// Set term filters
		normalizedFields := NormalizedDataFields(query.DataField, query.FieldWeights)
		if query.Type == Term && query.Value != nil && len(normalizedFields) > 0 {
			value := *query.Value
			valueAsArray, ok := value.([]interface{})
			dataField := normalizedFields[0].Field
			if ok {
				for _, val := range valueAsArray {
					// Use lower case for filter values
					filterValue := strings.ToLower(fmt.Sprintf("%v", val))
					termFilters = append(termFilters, TermFilter{
						Key:   dataField,
						Value: filterValue,
					})
				}
			} else {
				// Use lower case for filter value
				filterValue := strings.ToLower(fmt.Sprintf("%v", value))
				termFilters = append(termFilters, TermFilter{
					Key:   dataField,
					Value: filterValue,
				})
			}
		}
		queryEnvs.TermFilters = termFilters
	}
	return queryEnvs
}

// Returns the type of operation based on the relation defined in `react` prop
func getOperation(conjunction string) string {
	if conjunction == "and" {
		return "must"
	}
	if conjunction == "or" {
		return "should"
	}
	return "must_not"
}

// Returns the query instance by query id
func getQueryInstanceByID(id string, rsQuery RSQuery) *Query {
	for _, query := range rsQuery.Query {
		if query.ID != nil && *query.ID == id {
			return &query
		}
	}
	return nil
}

// Evaluate the react prop and adds the dependencies in query
func evalReactProp(query []interface{}, queryOptions *map[string]interface{}, conjunction string, react interface{}, rsQuery RSQuery) ([]interface{}, error) {
	nestedReact, isNestedReact := react.(map[string]interface{})
	if isNestedReact {
		var err error
		// handle react prop as struct
		if nestedReact["and"] != nil {
			query, err = evalReactProp(query, queryOptions, "and", nestedReact["and"], rsQuery)
			if err != nil {
				return query, err
			}
		}
		if nestedReact["or"] != nil {
			query, err = evalReactProp(query, queryOptions, "or", nestedReact["or"], rsQuery)
			if err != nil {
				return query, err
			}
		}
		if nestedReact["not"] != nil {
			query, err = evalReactProp(query, queryOptions, "not", nestedReact["not"], rsQuery)
			if err != nil {
				return query, err
			}
		}
		return query, nil
	} else {
		// handle react prop as an array
		reactAsArray, isArray := react.([]interface{})
		if isArray {
			var queryArr []interface{}
			for _, comp := range reactAsArray {
				componentID, isString := comp.(string)
				if isString {
					componentQueryInstance := getQueryInstanceByID(componentID, rsQuery)
					// ignore if query is not present for a component id i.e invalid component id has been used
					if componentQueryInstance != nil {
						queryOps, err := componentQueryInstance.buildQueryOptions()
						if err != nil {
							return query, err
						}
						// query options specific to a component for e.g `highlight`
						componentQueryOptions := getFilteredOptions(queryOps)
						// Apply custom query
						translatedQuery, options, err := componentQueryInstance.applyCustomQuery()
						if err != nil {
							return query, err
						}
						mergedQueryOptions := getFilteredOptions(mergeMaps(*queryOptions, mergeMaps(componentQueryOptions, options)))
						*queryOptions = mergedQueryOptions
						// Only apply query if not nil
						if !isNilInterface(*translatedQuery) {
							queryArr = append(queryArr, &translatedQuery)
						}
					}
				} else {
					return evalReactProp(query, queryOptions, "", comp, rsQuery)
				}
			}
			if len(queryArr) > 0 {
				// finally append the query
				boolQuery := createBoolQuery(getOperation(conjunction), queryArr)
				if boolQuery != nil {
					query = append(query, boolQuery)
				}
			}
		} else {
			// handle react prop as string
			reactAsString, isString := react.(string)
			if isString {
				componentQueryInstance := getQueryInstanceByID(reactAsString, rsQuery)
				if componentQueryInstance != nil {
					queryOps, err := componentQueryInstance.buildQueryOptions()
					if err != nil {
						return query, err
					}
					// query options specific to a component for e.g `highlight`
					componentQueryOptions := getFilteredOptions(queryOps)
					// Apply custom query
					translatedQuery, options, err := componentQueryInstance.applyCustomQuery()
					if err != nil {
						return query, err
					}
					mergedQueryOptions := getFilteredOptions(mergeMaps(*queryOptions, mergeMaps(componentQueryOptions, options)))
					*queryOptions = mergedQueryOptions
					if !isNilInterface(*translatedQuery) {
						shouldQuery := createBoolQuery(getOperation(conjunction), &translatedQuery)
						if shouldQuery != nil {
							query = append(query, shouldQuery)
						}
					}
				}
			}
		}
	}
	return query, nil
}

// Returns the queryDSL with react prop dependencies
func (query *Query) getQuery(rsQuery RSQuery) (*interface{}, map[string]interface{}, bool, error) {
	var finalQuery []interface{}
	var finalOptions = make(map[string]interface{})

	if query.React != nil {
		var err error
		finalQuery, err = evalReactProp(finalQuery, &finalOptions, "", *query.React, rsQuery)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, finalOptions, true, err
		}
	}
	if len(finalQuery) != 0 {
		if query.DefaultQuery != nil {
			defaultQuery := *query.DefaultQuery
			if defaultQuery["query"] != nil {
				finalQuery = append(finalQuery, defaultQuery["query"])
			}
		} else if query.Type == Search || query.Type == Suggestion {
			// Only apply query by `value` for search queries
			queryByType, err := query.generateQueryByType()
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, finalOptions, false, err
			}
			if queryByType != nil && !isNilInterface(*queryByType) {
				finalQuery = append(finalQuery, queryByType)
			}
		}
		var boolQuery interface{} = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": finalQuery,
			}}
		return &boolQuery, finalOptions, false, nil
	} else if query.DefaultQuery != nil {
		defaultQuery := *query.DefaultQuery
		if defaultQuery["query"] != nil {
			var query interface{} = defaultQuery["query"]
			return &query, finalOptions, false, nil
		}
	}
	queryByType, err := query.generateQueryByType()
	return queryByType, finalOptions, true, err
}

// removes some options from the query option added by react property
func getFilteredOptions(options map[string]interface{}) map[string]interface{} {
	filteredOptions := make(map[string]interface{})
	for k, v := range options {
		if !util.IsExists(k, EXCEPTION_KEYS_IN_QUERY) {
			filteredOptions[k] = v
		}
	}
	return filteredOptions
}

// Apply the custom query
func (query *Query) applyCustomQuery() (*interface{}, map[string]interface{}, error) {
	queryOptions := make(map[string]interface{})
	if query.CustomQuery != nil {
		customQuery := *query.CustomQuery
		if customQuery["query"] != nil {
			finalQuery := customQuery["query"]
			queryOptions = getFilteredOptions(customQuery)
			// filter query options keys
			return &finalQuery, queryOptions, nil
		}
		// filter query options keys
		queryOptions = getFilteredOptions(customQuery)
	}
	originalQuery, err := query.generateQueryByType()
	if err != nil {
		return nil, queryOptions, err
	}
	return originalQuery, queryOptions, nil
}

// Creates the bool query
func createBoolQuery(operation string, query interface{}) *map[string]interface{} {
	var resultQuery *map[string]interface{}

	queryAsArray, isArray := query.([]interface{})
	if (isArray && len(queryAsArray) != 0) || (query != nil) {
		resultQuery = &map[string]interface{}{
			"bool": map[string]interface{}{
				operation: query,
			},
		}
	}

	if operation == "should" && resultQuery != nil {
		tempResultQuery := *resultQuery
		tempQuery := tempResultQuery["bool"]
		shouldQuery, ok := tempQuery.(map[string]interface{})
		if ok {
			shouldQuery["minimum_should_match"] = 1
		}
		resultQuery = &map[string]interface{}{
			"bool": shouldQuery,
		}
	}

	return resultQuery
}

// To check if an item is present in a slice
func contains(s []interface{}, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// Merges the two maps, same keys will be overridden by the second map
func mergeMaps(x map[string]interface{}, y map[string]interface{}) map[string]interface{} {
	mergeMap := x
	for k, v := range y {
		mergeMap[k] = v
	}
	return mergeMap
}

func getValidInterval(interval *int, rangeValue RangeValue) int {
	normalizedInterval := 0
	if interval != nil {
		normalizedInterval = *interval
	}
	endValue := *rangeValue.End
	endValueAsFloat, ok := endValue.(float64)
	if !ok {
		return normalizedInterval
	}
	startValue := *rangeValue.Start
	startValueAsFloat, ok := startValue.(float64)
	if !ok {
		return normalizedInterval
	}
	min := math.Ceil(float64((endValueAsFloat - startValueAsFloat)) / 100)
	if min == 0 {
		min = 1
	}
	if normalizedInterval == 0 {
		return int(min)
	} else if normalizedInterval < int(min) {
		return int(min)
	}
	return normalizedInterval
}

func getQueryIds(rsQuery RSQuery) []string {
	var queryIds []string
	for _, query := range rsQuery.Query {
		if query.Execute == nil || *query.Execute {
			queryIds = append(queryIds, *query.ID)
		}
	}
	return queryIds
}

// isNilInterface checks if interface has a nil value
func isNilInterface(c interface{}) bool {
	return c == nil || reflect.ValueOf(c).IsNil()
}

// Makes the elasticsearch requests
func makeESRequest(ctx context.Context, url, method string, reqBody []byte) (*es7.Response, error) {
	esClient := util.GetClient7()
	requestOptions := es7.PerformRequestOptions{
		Method: method,
		Path:   url,
		Body:   string(reqBody),
	}
	response, err := esClient.PerformRequest(ctx, requestOptions)
	if err != nil {
		log.Errorln("Error while making request: ", err)
		return response, err
	}
	return response, nil
}

// To construct the data field string with field weight from `DataField` struct
func ParseDataFieldToString(dataFieldAsMap map[string]interface{}) *DataField {
	if dataFieldAsMap["field"] != nil {
		fieldAsString, ok := dataFieldAsMap["field"].(string)
		if ok && fieldAsString != "" {
			dataField := DataField{
				Field: fieldAsString,
			}
			if dataFieldAsMap["weight"] != nil {
				fieldWeight, ok := dataFieldAsMap["weight"].(float64)
				if ok {
					dataField.Weight = fieldWeight
				} else {
					fieldWeight, ok := dataFieldAsMap["weight"].(int)
					if ok {
						dataField.Weight = float64(fieldWeight)
					}
				}
			}
			return &dataField
		}
	}
	return nil
}

// The `dataField` property can be of following types
// - string
// - `DataField` struct with `field` and `weight` keys
// - Array of strings
// - Array of `DataField` struct
// - Array of strings and `DataField` struct
//
// The following function normalizes the dataField input into a array of strings
// It also supports the fieldWeights in old format
func NormalizedDataFields(dataField interface{}, fieldWeights []float64) []DataField {
	dataFieldAsString, ok := dataField.(string)
	if ok {
		dataField := DataField{
			Field: dataFieldAsString,
		}
		if len(fieldWeights) > 0 {
			dataField.Weight = fieldWeights[0]
		}
		return []DataField{dataField}
	}
	dataFieldAsMap, ok := dataField.(map[string]interface{})
	if ok {
		parsedField := ParseDataFieldToString(dataFieldAsMap)
		if parsedField != nil {
			return []DataField{*parsedField}
		}
	}
	dataFieldAsArray, ok := dataField.([]interface{})
	if ok {
		parsedFields := []DataField{}
		for index, field := range dataFieldAsArray {
			dataFieldAsString, ok := field.(string)
			if ok {
				dataField := DataField{
					Field: dataFieldAsString,
				}
				// Consider field weights to support older format
				if len(fieldWeights) > index {
					dataField.Weight = fieldWeights[index]
				}
				parsedFields = append(parsedFields, dataField)
			}
			dataFieldAsMap, ok := field.(map[string]interface{})
			if ok {
				parsedField := ParseDataFieldToString(dataFieldAsMap)
				if parsedField != nil {
					parsedFields = append(parsedFields, *parsedField)
				}
			}
		}
		return parsedFields
	}
	dataFieldAsArrayOfString, ok := dataField.([]string)
	if ok {
		parsedFields := []DataField{}
		for index, field := range dataFieldAsArrayOfString {
			dataField := DataField{
				Field: field,
			}
			// Consider field weights to support older format
			if len(fieldWeights) > index {
				dataField.Weight = fieldWeights[index]
			}
			parsedFields = append(parsedFields, dataField)
		}
		return parsedFields
	}
	return make([]DataField, 0)
}

// This function scans all the keys in the nested query
// and finds the top most value of a specified key
// For e.g to find the size defined for custom aggs where aggs key is unknown
func getSizeFromQuery(query *map[string]interface{}, key string) *interface{} {
	if query != nil {
		for k, v := range *query {
			// key found
			if k == key {
				return &v
			}

			valueAsMap, ok := v.(map[string]interface{})
			if ok {
				value := getSizeFromQuery(&valueAsMap, key)
				if value != nil {
					return value
				}
			}
		}
	}
	return nil
}

// SliceIndex provides a generic way to get an index of a slice
func sliceIndex(limit int, predicate func(i int) bool) int {
	for i := 0; i < limit; i++ {
		if predicate(i) {
			return i
		}
	}
	return -1
}

// a convenient min over integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// a convenient max over integers
func max(a, b int) int {
	if a == min(a, b) {
		return b
	}
	return a
}

// compressAndOrder compresses a string by removing stopwords, replacing diacritics, stemming and then orders its tokens in ascending
// It can be used to compare uniqueness of suggestions: e.g. "apple and iphone 12" is the same as a "apple iphone 12"
func CompressAndOrder(source string, config SuggestionsConfig) string {
	language := "english"
	if config.Language != nil {
		language = *config.Language
	}
	target := stemmedTokens(replaceDiacritics(removeStopwords(source, config)), language)
	sort.Strings(target)
	return strings.Join(target, " ")
}

// stemmedTokens returns stemmed tokens of a string
// based on the language. Includes language validation
func stemmedTokens(source string, language string) []string {
	tokens := strings.Split(source, " ")
	languages := []string{"english", "russian", "spanish", "french", "swedish", "norwegian"}
	index := sliceIndex(len(languages), func(i int) bool { return strings.Contains(languages[i], strings.ToLower(language)) })
	if index == -1 {
		language = "english"
	} else {
		language = languages[index]
	}
	var stemmedTokens []string
	for _, token := range tokens {
		// stem the token
		stemmedToken, err := snowball.Stem(token, language, false)
		if err == nil {
			stemmedTokens = append(stemmedTokens, stemmedToken)
		} else {
			// in case of an error, return the tokenized string
			stemmedTokens = append(stemmedTokens, token)
		}
	}
	return stemmedTokens
}

// removeStopwords removes stopwords including considering the suggestions config
func removeStopwords(value string, config SuggestionsConfig) string {
	ln := "en"
	if config.Language != nil && LanguagesToISOCode[*config.Language] != "" {
		ln = LanguagesToISOCode[*config.Language]
	}
	var userStopwords = make(map[string]string)
	// load any custom stopwords the user has
	// a highlighted phrase shouldn't be limited due to stopwords
	if config.ApplyStopwords != nil && *config.ApplyStopwords {
		// apply any custom stopwords
		if config.Stopwords != nil && len(*config.Stopwords) > 0 {
			for _, word := range *config.Stopwords {
				userStopwords[word] = ""
			}
		}
	}

	// we don't want to strip any numbers from the string
	stopwords.DontStripDigits()
	cleanContent := strings.Split(stopwords.CleanString(value, ln, true), " ")
	if len(userStopwords) > 0 {
		for i, token := range cleanContent {
			if _, ok := userStopwords[token]; ok {
				cleanContent[i] = " "
			}
		}
	}

	return normalizeValue(strings.Join(cleanContent, " "))
}

// normalizeValue changes a query's value to remove special chars and spaces
// e.g. Android - Black would be "android black"
// e.g. "Wendy's burger  " would be "wendys burger"
func normalizeValue(value string) string {
	// Trim the spaces and tokenize
	tokenizedValue := strings.Split(strings.TrimSpace(value), " ")
	var finalValue []string
	for _, token := range tokenizedValue {
		sT := sanitizeString(token)
		if len(sT) > 0 {
			finalValue = append(finalValue, strings.ToLower(sT))
		}
	}
	return strings.TrimSpace(strings.Join(finalValue, " "))
}

// A wrapper around normalizeValue to handle value transformation
// for search, suggestion types of queries at query generation time
func normalizeQueryValue(input *interface{}) (*interface{}, error) {
	if input == nil {
		return nil, nil
	}
	valueAsInterface := *input
	valueAsString, ok := valueAsInterface.(string)
	if !ok {
		// Return the error
		errMsg := "Expected query.value as string but got a non string type"
		return nil, errors.New(errMsg)
	}

	normalizedValue := sanitizeString(valueAsString)
	var outputValue interface{} = normalizedValue
	return &outputValue, nil
}

// Removes the extra spaces from a string
func removeSpaces(str string) string {
	return strings.Join(strings.Fields(str), " ")
}

// SanitizeString removes special chars and extra spaces from a string
// e.g. "android - black" becomes "android black"
// e.g. "android-black" doesn't change
func sanitizeString(str string) string {
	// remove extra spaces
	s := str
	tokenString := strings.Split(s, " ")
	specialChars := []string{"'", "/", "{", "(", "[", "-", "+", ".", "^", ":", ",", "]", ")", "}"}
	// Remove special characters when they're a token by themselves
	for i, token := range tokenString {
		if sliceIndex(len(specialChars), func(i int) bool { return token == specialChars[i] }) != -1 {
			// replace with a space instead
			tokenString[i] = " "
		}
	}
	return removeSpaces(strings.Join(tokenString, " "))
}

// Returns the parsed suggestion label to be compared for duplicate suggestions
func parseSuggestionLabel(label string, config SuggestionsConfig) string {
	// trim spaces
	parsedLabel := removeSpaces(label)
	// convert to lower case
	parsedLabel = removeStopwords(strings.ToLower(parsedLabel), config)
	stemLanguage := "english"
	if config.Language != nil {
		if util.Contains(StemLanguages, *config.Language) {
			stemLanguage = *config.Language
		}
	}
	stemmedTokens := stemmedTokens(parsedLabel, stemLanguage)
	// remove stopwords
	return removeSpaces(strings.Join(stemmedTokens, " "))
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

// replaces diacritics with their equivalent
func replaceDiacritics(query string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)
	queryKey, _, _ := transform.String(t, query)
	return queryKey
}

// Do this once for each unique policy, and use the policy for the life of the program
// Policy creation/editing is not safe to use in multiple goroutines
var p = bluemonday.StrictPolicy()

// extracts the string from HTML tags
func getTextFromHTML(body string) string {
	// The policy can then be used to sanitize lots of input and it is safe to use the policy in multiple goroutines
	html := p.Sanitize(
		body,
	)

	return html
}

// checks if a string is of type letter
func isLetter(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

// getPlural pluralizes a string passed as *interface type
func getPlural(input *interface{}) *interface{} {
	if input == nil {
		return nil
	}
	if rsPluralize == nil {
		rsPluralize = pluralize.NewClient()
	}
	// translate interface into string first
	valueAsInterface := *input
	valueAsString := sanitizeString(valueAsInterface.(string))

	var valueTokens = strings.Split(valueAsString, " ")
	var lastWord = valueTokens[len(valueTokens)-1]
	pluralString := valueAsString
	if isLetter(lastWord) {
		// is letter, can pluralize
		pluralString = rsPluralize.Plural(valueAsString)
	}
	// returning the plural string as *interface
	var returnValue interface{} = pluralString
	return &returnValue
}

// findMatch matches the user query against the field value to return scores and matched tokens
// This supports fuzzy matching in addition to normalized matching (i.e. after stopwords removal and stemming)
func findMatch(fieldValueRaw string, userQueryRaw string, config SuggestionsConfig) RankField {
	// remove stopwords from fieldValue and userQuery
	fieldValue := removeStopwords(fieldValueRaw, config)
	userQuery := removeStopwords(userQueryRaw, config)
	var rankField = RankField{
		fieldValue:    fieldValue,
		userQuery:     userQuery,
		score:         0,
		matchedTokens: nil,
	}
	stemLanguage := "english"
	if config.Language != nil {
		if util.Contains(StemLanguages, *config.Language) {
			stemLanguage = *config.Language
		}
	}
	fieldValues := strings.Split(fieldValue, " ")
	stemmedFieldValues := stemmedTokens(fieldValue, stemLanguage)
	stemmeduserQuery := stemmedTokens(userQuery, stemLanguage)
	foundMatches := make([]bool, len(stemmeduserQuery))

	for i, token := range stemmeduserQuery {
		// eliminate single char tokens from consideration
		if len(token) > 1 {
			foundMatch := false
			// start with the default distance of 1.0
			bestDistance := 1.0
			ranks := fuzzy.RankFindNormalizedFold(token, stemmedFieldValues)
			var bestTarget string
			for _, element := range ranks {
				switch element.Distance {
				case 0:
					// Perfect match, we can skip iteration and just return
					bestDistance = math.Min(0, bestDistance)
					foundMatch = true
					bestTarget = element.Target
				case 1:
					// 1 edit distance
					bestDistance = math.Min(1.0, bestDistance)
					foundMatch = true
					if bestTarget == "" {
						bestTarget = element.Target
					}
				}
			}
			matchIndex := sliceIndex(len(stemmedFieldValues), func(i int) bool {
				return stemmedFieldValues[i] == bestTarget
			})
			if matchIndex != -1 {
				rankField.matchedTokens = append(rankField.matchedTokens, fieldValues[matchIndex])
			}
			foundMatches[i] = foundMatch
			// token of user query matched one of the tokens of field values
			if foundMatch {
				rankField.score += 1.0 - (bestDistance / 2)
				// add score for a consecutive match
				if i > 0 {
					if foundMatches[i] && foundMatches[i-1] {
						rankField.score += 0.1
					}
				}
			}
		}
	}
	return rankField
}

// Util method to extract the fields from elasticsearch source object
// It can handle nested objects and arrays too.
// Example 1:
// Input: { a: 1, b: { b_1: 2, b_2: 3}}
// Output: ['a', 'b.b_1', 'b.b_2']
// Example 2:
// Input: { a: 1, b: [{c: 1}, {d: 2}, {c: 3}]}
// Output: ['a', 'b.c', 'b.d']
func extractFieldsFromSource(source map[string]interface{}) []string {
	dataFields := []string{}
	var sourceAsInterface interface{} = source
	dataFieldsMap := getFields(sourceAsInterface, "")
	for k := range dataFieldsMap {
		dataFields = append(dataFields, k)
	}
	return dataFields
}

// getFields is used by extractFieldsFromSource to recursively extract
// fields from the hit or a sub-part of the hit response tree
func getFields(source interface{}, prefix string) map[string]interface{} {
	dataFields := make(map[string]interface{})
	sourceAsMap, ok := source.(map[string]interface{})
	if ok {
		for field := range sourceAsMap {
			var key string
			if prefix != "" {
				key = prefix + "." + field
			} else {
				key = field
			}
			if sourceAsMap[field] != nil {
				mapValue, ok := sourceAsMap[field].(map[string]interface{})
				if ok {
					mergeMaps(dataFields, getFields(mapValue, key))
				} else {
					mapValueAsArray, ok := sourceAsMap[field].([]interface{})
					if ok {
						mergeMaps(dataFields, getFields(mapValueAsArray, key))
					} else {
						mergeMaps(dataFields, map[string]interface{}{
							key: true,
						})
					}
				}
			}
		}
	} else {
		sourceAsArray, ok := source.([]interface{})
		if ok {
			for field := range sourceAsArray {
				var key string
				if prefix != "" {
					key = prefix
				} else {
					key = strconv.Itoa(field)
				}
				if sourceAsArray[field] != nil {
					mapValue, ok := sourceAsArray[field].(map[string]interface{})
					if ok {
						mergeMaps(dataFields, getFields(mapValue, key))
					} else {
						mapValueAsArray, ok := sourceAsArray[field].([]interface{})
						if ok {
							mergeMaps(dataFields, getFields(mapValueAsArray, key))
						} else {
							mergeMaps(dataFields, map[string]interface{}{
								key: true,
							})
						}
					}
				}
			}
		}
	}

	return dataFields
}

// addFieldHighlight highlights the fields of the hit based on the highlight value in a new ParsedSource key
func addFieldHighlight(source ESDoc) ESDoc {
	source.ParsedSource = make(map[string]interface{})
	// clone map
	for k, v := range source.Source {
		source.ParsedSource[k] = v
	}

	if source.Highlight != nil {
		for highlightItem, highlightedValue := range source.Highlight {
			highlightValueArray, ok := highlightedValue.([]interface{})
			if ok && len(highlightValueArray) > 0 {
				highlightValue := highlightValueArray[0]
				source.ParsedSource[highlightItem] = highlightValue
			}
		}
	}
	return source
}
