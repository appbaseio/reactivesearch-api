package rsapi

import (
	"encoding/json"
	"fmt"
)

// QueryType represents the type of query
type QueryType int

const (
	Search QueryType = iota
	Term
	Range
	Geo
)

// String is the implementation of Stringer interface that returns the string representation of QueryType type.
func (o QueryType) String() string {
	return [...]string{
		"search",
		"term",
		"range",
		"geo",
	}[o]
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
	default:
		return nil, fmt.Errorf("invalid queryType encountered: %v", o)
	}
	return json.Marshal(queryType)
}

// SortBy represents the sort by property
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
	ID                *string                 `json:"id,omitempty"` // component id
	Type              QueryType               `json:"type,omitempty"`
	React             *map[string]interface{} `json:"react,omitempty"`
	QueryFormat       *QueryFormat            `json:"queryFormat,omitempty"`
	DataField         []string                `json:"dataField,omitempty"`
	CategoryField     *string                 `json:"categoryField,omitempty"`
	CategoryValue     *interface{}            `json:"categoryValue,omitempty"`
	FieldWeights      []int                   `json:"fieldWeights,omitempty"`
	NestedField       *string                 `json:"nestedField,omitempty"`
	From              *int                    `json:"from,omitempty"`
	Size              *int                    `json:"size,omitempty"`
	SortBy            *SortBy                 `json:"sortBy,omitempty"`
	Value             *interface{}            `json:"value,omitempty"` // either string or Array of string
	AggregationField  *string                 `json:"aggregationField,omitempty"`
	After             *map[string]interface{} `json:"after,omitempty"`
	IncludeNullValues *bool                   `json:"includeNullValues,omitempty"`
	IncludeFields     *[]string               `json:"includeFields,omitempty"`
	ExcludeFields     *[]string               `json:"excludeFields,omitempty"`
	Fuzziness         interface{}             `json:"fuzziness,omitempty"` // string or int
	SearchOperators   *bool                   `json:"searchOperators,omitempty"`
	Highlight         *bool                   `json:"highlight,omitempty"`
	HighlightField    []string                `json:"highlightField,omitempty"`
	CustomHighlight   *map[string]interface{} `json:"customHighlight,omitempty"`
	Interval          *int                    `json:"interval,omitempty"`
	Aggregations      *[]string               `json:"aggregations,omitempty"`
	MissingLabel      string                  `json:"missingLabel,omitempty"`
	ShowMissing       bool                    `json:"showMissing,omitempty"`
	DefaultQuery      *map[string]interface{} `json:"defaultQuery,omitempty"`
	CustomQuery       *map[string]interface{} `json:"customQuery,omitempty"`
	Execute           *bool                   `json:"execute,omitempty"`
	EnableSynonyms    *bool                   `json:"enableSynonyms,omitempty"`
	SelectAllLabel    *string                 `json:"selectAllLabel,omitempty"`
	Pagination        *bool                   `json:"pagination,omitempty"`
	QueryString       *bool                   `json:"queryString,omitempty"`
}

// Settings represents the search settings
type Settings struct {
	RecordAnalytics  *bool              `json:"recordAnalytics,omitempty"`
	UserID           *string            `json:"userId,omitempty"`
	CustomEvents     *map[string]string `json:"customEvents,omitempty"`
	EnableQueryRules *bool              `json:"enableQueryRules,omitempty"`
}

// RSQuery represents the request body
type RSQuery struct {
	Query    []Query   `json:"query,omitempty"`
	Settings *Settings `json:"settings,omitempty"`
}
