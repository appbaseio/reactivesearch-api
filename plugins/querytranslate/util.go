package querytranslate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/bbalet/stopwords"
	pluralize "github.com/gertd/go-pluralize"
	"github.com/invopop/jsonschema"
	"github.com/kljensen/snowball"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"github.com/microcosm-cc/bluemonday"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var ES_MOCKED_RESPONSE = map[string]interface{}{
	"took":      0,
	"timed_out": false,
	"_shards": map[string]interface{}{
		"total":      1,
		"successful": 1,
		"skipped":    0,
		"failed":     0,
	},
	"hits": map[string]interface{}{
		"total": map[string]interface{}{
			"value":    0,
			"relation": "eq",
		},
		"max_score": nil,
		"hits":      make([]interface{}, 0),
	},
	"status": 200,
}

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
	Featured
	EndpointSuggestion
)

// String is the implementation of Stringer interface that returns the string representation of SuggestionType type.
func (o SuggestionType) String() string {
	return [...]string{
		"index",
		"popular",
		"recent",
		"promoted",
		"featured",
		"endpoint",
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
	case Featured.String():
		*o = Featured
	case EndpointSuggestion.String():
		*o = EndpointSuggestion
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
	case Featured:
		suggestionType = Featured.String()
	case EndpointSuggestion:
		suggestionType = EndpointSuggestion.String()
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

// JSONSchema will return the jsonschema for QueryType
func (QueryType) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []interface{}{
			Search.String(),
			Term.String(),
			Range.String(),
			Geo.String(),
			Suggestion.String(),
		},
		Title:       "type",
		Description: "type of query",
	}
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

// JSONSchema will return the jsonschema for SortBy
func (SortBy) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []interface{}{
			Asc.String(),
			Desc.String(),
			Count.String(),
		},
		Title:       "sortBy",
		Description: "order to sort by",
	}
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

// JSONSchema will return the jsonschema for QueryFormat
func (QueryFormat) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []interface{}{
			Or.String(),
			And.String(),
		},
		Title:       "queryFormat",
		Description: "operators to use for joining queries",
	}
}

// Backend will be the backend to be used for the knn
// response stage changes.
type Backend int

const (
	ElasticSearch Backend = iota
	OpenSearch
	MongoDB
	Solr
	Zinc
	MarkLogic
)

// String returns the string representation
// of the Backend
func (b Backend) String() string {
	switch b {
	case ElasticSearch:
		return "elasticsearch"
	case OpenSearch:
		return "opensearch"
	case MongoDB:
		return "mongodb"
	case Solr:
		return "solr"
	case Zinc:
		return "zinc"
	case MarkLogic:
		return "marklogic"
	}
	return ""
}

// UnmarshalJSON is the implementation of Unmarshaler interface to unmarshal the Backend
func (b *Backend) UnmarshalJSON(bytes []byte) error {
	var knnBackend string
	err := json.Unmarshal(bytes, &knnBackend)
	if err != nil {
		return err
	}

	switch knnBackend {
	case OpenSearch.String():
		*b = OpenSearch
	case ElasticSearch.String():
		*b = ElasticSearch
	case MongoDB.String():
		*b = MongoDB
	case Solr.String():
		*b = Solr
	case Zinc.String():
		*b = Zinc
	case MarkLogic.String():
		*b = MarkLogic
	default:
		return fmt.Errorf("invalid kNN backend passed: %s", knnBackend)
	}

	return nil
}

// MarshalJSON is the implementation of the Marshaler interface to marshal the Backend
func (b Backend) MarshalJSON() ([]byte, error) {
	knnBackend := b.String()

	if knnBackend == "" {
		return nil, fmt.Errorf("invalid kNN backend passed: %s", knnBackend)
	}

	return json.Marshal(knnBackend)
}

func (b Backend) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []interface{}{
			ElasticSearch.String(),
			OpenSearch.String(),
			MongoDB.String(),
			Solr.String(),
			Zinc.String(),
			MarkLogic.String(),
		},
		Title:       "Backend",
		Description: "Backend that ReactiveSearch will use",
	}
}

// DeepPaginationConfig Struct
type DeepPaginationConfig struct {
	// The `cursor` value will map according to the
	// backend.
	//
	// - ES: `search_after` ([$cursor])
	// - Solr: `cursorMark` $cursor
	Cursor *string `json:"cursor,omitempty"`
}

// Endpoint struct
type Endpoint struct {
	URL     *string            `json:"url,omitempty"`
	Method  *string            `json:"method,omitempty"`
	Headers *map[string]string `json:"headers,omitempty"`
	Body    *interface{}       `json:"body,omitempty"`
}

// Query represents the query object
type Query struct {
	ID                          *string                     `json:"id,omitempty" jsonschema:"title=id,description=ID of the query,required" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"` // component id
	Type                        QueryType                   `json:"type,omitempty" jsonschema:"title=type,description=type of query" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	React                       *map[string]interface{}     `json:"react,omitempty" jsonschema:"title=react,description=which queries to react the current query with" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	QueryFormat                 *string                     `json:"queryFormat,omitempty" jsonschema:"title=queryFormat,description=the operator to join multiple values in the query.value field" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	DataField                   interface{}                 `json:"dataField,omitempty" jsonschema:"title=dataField,description=fields to run the query term on" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	CategoryField               *string                     `json:"categoryField,omitempty" jsonschema:"title=categoryField,description=" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	CategoryValue               *interface{}                `json:"categoryValue,omitempty" jsonschema:"title=categoryValue,description=" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	FieldWeights                []float64                   `json:"fieldWeights,omitempty" jsonschema:"title=fieldWeights,description=(deprecated) weights of the data fields" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	NestedField                 *string                     `json:"nestedField,omitempty" jsonschema:"title=nestedField,description=" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	From                        *int                        `json:"from,omitempty" jsonschema:"title=from,description=index from which the results should start from" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	Size                        *int                        `json:"size,omitempty" jsonschema:"title=size,description=size of the results returned" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	AggregationSize             *int                        `json:"aggregationSize,omitempty" jsonschema:"title=aggregationSize,description=size of the aggregation" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	SortBy                      *SortBy                     `json:"sortBy,omitempty" jsonschema:"title=sortBy,description=sort order for the results" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	SortField                   *interface{}                `json:"sortField,omitempty" jsonschema:"title=sortField,description=field(s) to run the sorting on" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	Value                       *interface{}                `json:"value,omitempty" jsonschema:"title=value,description=value for the query. Can be string or array of strings" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"` // either string or Array of string
	AggregationField            *string                     `json:"aggregationField,omitempty" jsonschema:"aggregationField,description=field for doing the aggregation on" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	After                       *map[string]interface{}     `json:"after,omitempty" jsonschema:"title=after,description=pagination for aggregations" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	IncludeNullValues           *bool                       `json:"includeNullValues,omitempty" jsonschema:"title=includeNullValues,description=whether or not to include null values" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	IncludeFields               *[]string                   `json:"includeFields,omitempty" jsonschema:"title=includeFields,description=indicates which dataFields to include in search results" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	ExcludeFields               *[]string                   `json:"excludeFields,omitempty" jsonschema:"title=excludeFields,description=indicates which dataFields to exclude in search results" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	Fuzziness                   interface{}                 `json:"fuzziness,omitempty" jsonschema:"title=fuzziness,description=indicates the fuzziness of the query" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"` // string or int
	SearchOperators             *bool                       `json:"searchOperators,omitempty" jsonschema:"title=searchOperators,description=use special characters in the search query to enable advanced search behavior" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	Highlight                   *bool                       `json:"highlight,omitempty" jsonschema:"title=highlight,description=whether or not to enable highlighting of results" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	HighlightField              []string                    `json:"highlightField,omitempty" jsonschema:"title=highlightField,description=fields to highlight in the results" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	CustomHighlight             *map[string]interface{}     `json:"customHighlight,omitempty" jsonschema:"title=customHighlight,description=(deprecated) same as highlightConfig" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	HighlightConfig             *map[string]interface{}     `json:"highlightConfig,omitempty" jsonschema:"title=highlightConfig,description=settings for highlighting of results" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	Interval                    *int                        `json:"interval,omitempty" jsonschema:"title=interval,description=histogram bar interval, applicable only when aggregations are set to histogram" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	Aggregations                *[]string                   `json:"aggregations,omitempty" jsonschema:"title=aggregations,description=utilize the built-in aggregations for range type of queries" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	MissingLabel                string                      `json:"missingLabel,omitempty" jsonschema:"title=missingLabel,description=custom label to show when showMissing is set to true" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	ShowMissing                 *bool                       `json:"showMissing,omitempty" jsonschema:"title=showMissing,description=whether or not to show missing results" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	DefaultQuery                *map[string]interface{}     `json:"defaultQuery,omitempty" jsonschema:"title=defaultQuery,description=customize the source query. This doesn't get leaked to other queries unlike customQuery" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	CustomQuery                 *map[string]interface{}     `json:"customQuery,omitempty" jsonschema:"title=customQuery,description=query to be used by dependent queries specified using the react property" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	Execute                     *bool                       `json:"execute,omitempty" jsonschema:"title=execute,description=whether or not to execute the query" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	EnableSynonyms              *bool                       `json:"enableSynonyms,omitempty" jsonschema:"title=enableSynonyms,description=control the synonyms behavior for a particular query" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	SelectAllLabel              *string                     `json:"selectAllLabel,omitempty" jsonschema:"title=selectAllLabel,description=allows adding a new property in the list with a particular value such that when selected, it is similar to that label" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	Pagination                  *bool                       `json:"pagination,omitempty" jsonschema:"title=pagination,description=enable pagination for term type of queries" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	QueryString                 *bool                       `json:"queryString,omitempty" jsonschema:"title=queryString,description=whether or not to allow creating a complex search that includes wildcard characters, searches across multiple fields, and more" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	RankFeature                 *map[string]RankFunction    `json:"rankFeature,omitempty" jsonschema:"title=rankFeature,description=boost relevant score of documents based on rank_feature fields" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	DistinctField               *string                     `json:"distinctField,omitempty" jsonschema:"title=distinctField,description=returns only distinct value documents for the specified field" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	DistinctFieldConfig         *map[string]interface{}     `json:"distinctFieldConfig,omitempty" jsonschema:"title=distinctFieldConfig,description=additional options to the distinctField property" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	Index                       *string                     `json:"index,omitempty" jsonschema:"title=index,description=explicitly specify an index to run the query on" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=opensearch"`
	EnableRecentSuggestions     *bool                       `json:"enableRecentSuggestions,omitempty" jsonschema:"title=enableRecentSuggestions,description=whether or not to enable recent suggestions" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	RecentSuggestionsConfig     *RecentSuggestionsOptions   `json:"recentSuggestionsConfig,omitempty" jsonschema:"title=recentSuggestionsConfig,description=additional options for getting recent suggestions" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	EnablePopularSuggestions    *bool                       `json:"enablePopularSuggestions,omitempty" jsonschema:"title=enablePopularSuggestions,description=whether or not to enable popular suggestions" jsonschema_extras:"engine=elasticsearch,engine=mongodb,engine=solr,engine=opensearch"`
	PopularSuggestionsConfig    *PopularSuggestionsOptions  `json:"popularSuggestionsConfig,omitempty" jsonschema:"title=popularSuggestionsConfig,description=additional options for getting popular suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	ShowDistinctSuggestions     *bool                       `json:"showDistinctSuggestions,omitempty" jsonschema:"title=showDistinctSuggestions,description=whether or not to show distinct suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	EnablePredictiveSuggestions *bool                       `json:"enablePredictiveSuggestions,omitempty" jsonschema:"title=enablePredictiveSuggestions,description=predicts the next relevant words from the value of a field based on the search query typed by the user" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	MaxPredictedWords           *int                        `json:"maxPredictedWords,omitempty" jsonschema:"title=maxPredictedWords,description=specify the the maximum number of relevant words that are predicted" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	URLField                    *string                     `json:"urlField,omitempty" jsonschema:"title=urlField,description=convenience prop that allows returning the URL value in the suggestion's response" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	ApplyStopwords              *bool                       `json:"applyStopwords,omitempty" jsonschema:"title=applyStopwords,description=whether or not predict a suggestion which starts or ends with a stopword" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	Stopwords                   *[]string                   `json:"customStopwords,omitempty" jsonschema:"title=customStopwords,description=list of custom stopwords" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	SearchLanguage              *string                     `json:"searchLanguage,omitempty" jsonschema:"title=searchLanguage,description=used to apply language specific stopwords for predictive suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	CalendarInterval            *string                     `json:"calendarinterval,omitempty" jsonschema:"title=calendarInterval,description=set the histogram bar interval when range value is of type date" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	Script                      *string                     `json:"script,omitempty" jsonschema:"title=script,description=indicates the script to run while reordering the results" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	QueryVector                 *[]float64                  `json:"queryVector,omitempty" jsonschema:"title=queryVector,description=specify a vector to match for the reordering the results using kNN" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	VectorDataField             *string                     `json:"vectorDataField,omitempty" jsonschema:"title=vectorDataField,description=field in the index to be used to reorder the results using kNN" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	Candidates                  *int                        `json:"candidates,omitempty" jsonschema:"title=candidates,description=indicates the number of candidates to consider while using the script_score functionality to reorder the results using kNN" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	EnableFeaturedSuggestions   *bool                       `json:"enableFeaturedSuggestions,omitempty" jsonschema:"title=enableFeaturedSuggestions,description=whether or not to enable featured suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	FeaturedSuggestionsConfig   *FeaturedSuggestionsOptions `json:"featuredSuggestionsConfig,omitempty" jsonschema:"title=featuredSuggestionsConfig,description=additional options to specify for featured suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	EnableIndexSuggestions      *bool                       `json:"enableIndexSuggestions,omitempty" jsonschema:"title=enableIndexSuggestions,description=whether or not to enable index suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	EnableEndpointSuggestions   *bool                       `json:"enableEndpointSuggestions,omitempty" jsonschema:"title=enableEndpointSuggestions,description=whether or not to enable endpoint suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	IndexSuggestionsConfig      *IndexSuggestionsOptions    `json:"indexSuggestionsConfig,omitempty" jsonschema:"title=indexSuggestionsConfig,description=additional options to specify for index suggestions" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	DeepPagination              *bool                       `json:"deepPagination,omitempty" jsonschema:"title=deepPagination,description=whether or not the enable deep pagination of results" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	DeepPaginationConfig        *DeepPaginationConfig       `json:"deepPaginationConfig,omitempty" jsonschema:"title=deepPaginationConfig,description=additional options for deepPagination for it to work properly" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	Endpoint                    *Endpoint                   `json:"endpoint,omitempty" jsonschema:"title=endpoint,description=endpoint and other details where the query should be hit" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	IncludeValues               *[]string                   `json:"includeValues,omitempty" jsonschema:"title=includeValues,description=values to include in term queries" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	ExcludeValues               *[]string                   `json:"excludeValues,omitempty" jsonschema:"title=excludeValues,description=values to exclude in term queries" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	SearchBoxId                 *string                     `json:"searchboxId,omitempty" jsonschema:"title=searchboxId,description=searchbox id for a suggestion query" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	Range                       *interface{}                `json:"range,omitempty" jsonschema:"title=range,description=range value to filter the histogram aggregations" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
}

type DataField struct {
	Field  string  `json:"field"`
	Weight float64 `json:"weight,omitempty"`
}

// Settings represents the search settings
type Settings struct {
	RecordAnalytics       *bool                   `json:"recordAnalytics,omitempty" jsonschema:"title=recordAnalytics,description=whether or not to record analytics for the current request" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
	UserID                *string                 `json:"userId,omitempty" jsonschema:"title=userId,description=user ID that will be used to record the analytics" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	CustomEvents          *map[string]interface{} `json:"customEvents,omitempty" jsonschema:"title=customEvents,description=custom events that can be used to build own analytics on top of ReactiveSearch analytics" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	EnableQueryRules      *bool                   `json:"enableQueryRules,omitempty" jsonschema:"title=enableQueryRules,description=whether or not to apply the query rules for the current request" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	EnableSearchRelevancy *bool                   `json:"enableSearchRelevancy,omitempty" jsonschema:"title=enableSearchRelevancy,description=whether or not to apply search relevancy for the current request" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	UseCache              *bool                   `json:"useCache,omitempty" jsonschema:"title=useCache,description=whether or not to use cache for the current request" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	QueryRule             *map[string]interface{} `json:"queryRule,omitempty" jsonschema:"title=queryRule,description=" jsonschema_extras:"engine=elasticsearch,engine=opensearch"`
	Backend               *Backend                `json:"backend,omitempty" jsonschema:"title=backend,description=backend to use for the current request" jsonschema_extras:"engine=elasticsearch,engine=solr,engine=opensearch"`
}

// RSQuery represents the request body
type RSQuery struct {
	Query    []Query                 `json:"query,omitempty" jsonschema:"title=query,description=The array of queries to execute,required"`
	Settings *Settings               `json:"settings,omitempty" jsonschema:"title=settings,description=Settings for the request being made"`
	Metadata *map[string]interface{} `json:"metadata,omitempty" jsonschema:"title=metadata,description=Metadata for the request being made"`
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
		if query.Value != nil {
			if query.Type == Search {
				value := *query.Value
				valueAsString, ok := value.(string)
				if ok {
					// Use query in lower case
					queryLowerCase := strings.ToLower(valueAsString)
					queryEnvs.Query = &queryLowerCase
				} else {
					valueAsArray, ok := value.([]interface{})
					if ok {
						var valueAsArrayString []string
						for _, v := range valueAsArray {
							valAsString, ok := v.(string)
							if ok && strings.TrimSpace(valAsString) != "" {
								valueAsArrayString = append(valueAsArrayString, valAsString)
							}
						}
						if len(valueAsArrayString) != 0 {
							queryLowerCase := strings.ToLower(strings.Join(valueAsArrayString, ","))
							queryEnvs.Query = &queryLowerCase
						}

					}
				}
			} else if query.Type == Suggestion {
				value := *query.Value
				valueAsString, ok := value.(string)
				if ok {
					// Use query in lower case
					queryLowerCase := strings.ToLower(valueAsString)
					queryEnvs.Query = &queryLowerCase
				}
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
func evalReactProp(query []interface{}, queryOptions *map[string]interface{}, conjunction string, react interface{}, rsQuery RSQuery, buildByTypeFunc QueryByType) ([]interface{}, error) {
	nestedReact, isNestedReact := react.(map[string]interface{})
	if isNestedReact {
		var err error
		// handle react prop as struct
		if nestedReact["and"] != nil {
			query, err = evalReactProp(query, queryOptions, "and", nestedReact["and"], rsQuery, buildByTypeFunc)
			if err != nil {
				return query, err
			}
		}
		if nestedReact["or"] != nil {
			query, err = evalReactProp(query, queryOptions, "or", nestedReact["or"], rsQuery, buildByTypeFunc)
			if err != nil {
				return query, err
			}
		}
		if nestedReact["not"] != nil {
			query, err = evalReactProp(query, queryOptions, "not", nestedReact["not"], rsQuery, buildByTypeFunc)
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
						translatedQuery, options, err := componentQueryInstance.applyCustomQuery(buildByTypeFunc)
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
					return evalReactProp(query, queryOptions, "", comp, rsQuery, buildByTypeFunc)
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
					translatedQuery, options, err := componentQueryInstance.applyCustomQuery(buildByTypeFunc)
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

// EvalReactProp will evaluate the react prop and add dependencies in the query
func EvalReactProp(query []interface{}, queryOptions *map[string]interface{}, conjunction string, react interface{}, rsQuery RSQuery, buildByTypeFunc QueryByType) ([]interface{}, error) {
	return evalReactProp(query, queryOptions, conjunction, react, rsQuery, buildByTypeFunc)
}

// Returns the queryDSL with react prop dependencies
func (query *Query) getQuery(rsQuery RSQuery) (*interface{}, map[string]interface{}, bool, error) {
	var finalQuery []interface{}
	var finalOptions = make(map[string]interface{})

	if query.React != nil {
		var err error
		finalQuery, err = evalReactProp(finalQuery, &finalOptions, "", *query.React, rsQuery, generateQueryByType)
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
func (query *Query) applyCustomQuery(byTypeFunc QueryByType) (*interface{}, map[string]interface{}, error) {
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
	originalQuery, err := byTypeFunc(query)
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
	normalizedInterval := 1
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
	if normalizedInterval == 1 {
		return int(min)
	} else if normalizedInterval < int(min) {
		return int(min)
	}
	return normalizedInterval
}

func (query *Query) shouldExecuteQuery() bool {
	// don't execute query if index suggestions are disabled
	if query.Type == Suggestion &&
		query.EnableIndexSuggestions != nil && query.Endpoint == nil &&
		!*query.EnableIndexSuggestions {
		return false
	}
	// don't execute query if endpoint suggestions are disabled
	if query.Type == Suggestion &&
		query.EnableEndpointSuggestions != nil &&
		!*query.EnableEndpointSuggestions {
		return false
	}
	if query.Execute != nil {
		return *query.Execute
	}
	return true
}

func GetQueryIds(rsQuery RSQuery) []string {
	var queryIds []string
	for _, query := range rsQuery.Query {
		// If endpoint is passed, execute is set as False
		if query.shouldExecuteQuery() && query.Endpoint == nil {
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
func makeESRequest(ctx context.Context, url, method string, reqBody []byte, params url.Values) (*es7.Response, error) {
	esClient := util.GetClient7()
	filteredParams := params
	for k := range filteredParams {
		if k == "preference" {
			filteredParams.Del("preference")
		}
	}
	requestOptions := es7.PerformRequestOptions{
		Method: method,
		Path:   url,
		Body:   string(reqBody),
		Params: filteredParams,
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
		errMsg := "Expected query.value to be of type string, but got a different type"
		return nil, errors.New(errMsg)
	}

	normalizedValue := sanitizeString(valueAsString)
	var outputValue interface{} = normalizedValue
	return &outputValue, nil
}

// NormalizeQueryValue is A wrapper around normalizeValue to handle
// value transformation for search, suggestion types of queries
// at query generation time
func NormalizeQueryValue(input *interface{}) (*interface{}, error) {
	return normalizeQueryValue(input)
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
func GetTextFromHTML(body string) string {
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
func FindMatch(fieldValueRaw string, userQueryRaw string, config SuggestionsConfig) RankField {
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

// ParseSortField will parse the sortField based on the values
// passed.
func ParseSortField(query Query, defaultSortBy SortBy) (map[string]SortBy, error) {
	sortFieldParsed := make(map[string]SortBy)

	// Parse as array of interface
	sortFieldAsArr, asArrOk := (*query.SortField).([]interface{})
	if asArrOk {
		// Parse the array and return detail accordingly
		// The value can be both a map  as well as a string.

		for sortFieldIndex, sortFieldEach := range sortFieldAsArr {
			// Try to parse it as an object
			fieldEachAsMap, asMapOk := sortFieldEach.(map[string]interface{})
			if !asMapOk {
				// Try to parse as string.
				fieldAsString, asStrOk := sortFieldEach.(string)

				// If it's not a string either, invalid type is passed.
				if !asStrOk {
					return sortFieldParsed, fmt.Errorf("invalid type passed in sortField array at index: %d", sortFieldIndex)
				}

				// If string is okay, add it to map and continue
				sortByToUse := defaultSortBy
				if fieldAsString == "_score" {
					sortByToUse = Desc
				}

				sortFieldParsed[fieldAsString] = sortByToUse

				continue
			}

			// If passed as map, parse it properly.
			// Make sure only one key is parsed from the index.
			parseCount := 0

			for key, value := range fieldEachAsMap {
				if parseCount > 1 {
					break
				}

				// Parse the value as sortBy, if fails then raise an error.
				valueAsStr, valueAsStrOk := value.(string)
				if !valueAsStrOk {
					return sortFieldParsed, fmt.Errorf("invalid sort value passed for index `%d` and key: `%s`", sortFieldIndex, key)
				}

				type CustomSortByContainer struct {
					SortBy *SortBy `json:"sortBy,omitempty"`
				}

				sortByTypeStr := fmt.Sprintf("{\"sortBy\": \"%s\"}", valueAsStr)

				newCustomType := new(CustomSortByContainer)
				unmarshalErr := json.Unmarshal([]byte(sortByTypeStr), &newCustomType)

				if unmarshalErr != nil {
					return sortFieldParsed, fmt.Errorf("invalid value passed for `sortBy` for index `%d` and key: `%s`", sortFieldIndex, key)
				}

				// If value is okay, add it to map.
				sortFieldParsed[key] = *newCustomType.SortBy
				parseCount += 1
			}
		}

		// Return the parsed map
		return sortFieldParsed, nil
	}

	// Parse as string
	sortFieldAsStr, asStrOk := (*query.SortField).(string)
	if !asStrOk {
		return sortFieldParsed, fmt.Errorf("invalid value passed for `sortField`, only array and string are accepted!")
	}

	// Parse the string and return accordingly.
	sortByToUse := defaultSortBy
	if sortFieldAsStr == "_score" {
		sortByToUse = Desc
	}
	sortFieldParsed[sortFieldAsStr] = sortByToUse

	return sortFieldParsed, nil
}

// extractIDFromPreference will extract the query ID from the preference
// string passed.
//
// Idea is to split it based on underscore `_` and remove the last element
// and join it back using underscore `_`
func extractIDFromPreference(preference string) string {
	textSplitted := strings.Split(preference, "_")

	textSplitted = textSplitted[:len(textSplitted)-1]

	return strings.Join(textSplitted, "_")
}

// GetReactiveSearchSchema will return the schema of RS API as bytes
func GetReactiveSearchSchema() ([]byte, error) {
	schema := GetReflactor().Reflect(&RSQuery{})
	schemaMarshalled, marshalErr := schema.MarshalJSON()

	if marshalErr != nil {
		return nil, marshalErr
	}

	// Unmarshal and inject
	var injectErr error
	schemaMarshalled, injectErr = injectExtrasToSchema(schemaMarshalled, *schema)

	return schemaMarshalled, injectErr
}

var jsonSchemaInstance *jsonschema.Reflector
var jsonSchemaInstanceOnce sync.Once

func GetReflactor() *jsonschema.Reflector {
	jsonSchemaInstanceOnce.Do(func() {
		r := new(jsonschema.Reflector)
		r.ExpandedStruct = true
		r.AllowAdditionalProperties = false
		r.DoNotReference = true
		r.RequiredFromJSONSchemaTags = true
		jsonSchemaInstance = r
	})
	return jsonSchemaInstance
}
