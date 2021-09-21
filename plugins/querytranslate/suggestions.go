package querytranslate

import (
	"encoding/json"
	"fmt"
)

type SuggestionType int

const (
	Index SuggestionType = iota
	Popular
	Recent
)

// String is the implementation of Stringer interface that returns the string representation of SuggestionType type.
func (o SuggestionType) String() string {
	return [...]string{
		"index",
		"popular",
		"recent",
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
	default:
		return fmt.Errorf("invalid suggestionType encountered: %v", suggestionType)
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
	default:
		return nil, fmt.Errorf("invalid suggestionType encountered: %v", o)
	}
	return json.Marshal(suggestionType)
}

// SuggestionHIT represents the structure of the suggestion object in RS API response
type SuggestionHIT struct {
	Value    string         `json:"value"`
	Label    string         `json:"label"`
	URL      *string        `json:"url"`
	Type     SuggestionType `json:"_suggestion_type"`
	Category *string        `json:"_category"`
	Count    *int           `json:"_count"`
	// ES response properties
	Index  *string                `json:"_index"`
	Score  *string                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}

type SuggestionHitResponse struct {
	Total    interface{}     `json:"total"`
	MaxScore interface{}     `json:"max_score"`
	Hits     []SuggestionHIT `json:"hits"`
}

// Response of the suggestions API similar to the ES response
type SuggestionESResponse struct {
	Took int                   `json:"took"`
	Hits SuggestionHitResponse `json:"hits"`
}

// RecentSuggestionsOptions represents the options to configure recent suggestions
type RecentSuggestionsOptions struct {
	Size  *int    `json:"size,omitempty"`
	Index *string `json:"index,omitempty"`
}

// PopularSuggestionsOptions represents the options to configure popular suggestions
type PopularSuggestionsOptions struct {
	Size       *int    `json:"size,omitempty"`
	Index      *string `json:"index,omitempty"`
	ShowGlobal *bool   `json:"showGlobal,omitempty"`
}
