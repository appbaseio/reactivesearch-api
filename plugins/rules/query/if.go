package query

import (
	"encoding/json"
	"fmt"
)

// If represents the rule condition that is executed against the
// search requests. If the search request meets the rule condition, then
// the rule consequence is triggered.
type If struct {
	Query    *string   `json:"query"`
	Operator *Operator `json:"operator"`
	WebHook  *WebHook  `json:"webhook,omitempty"`
}

// Operator represents the criterias that are matched against the search requests.
type Operator int

// WebHook will contain information about the webhook which has to be called.
// If headers are not provided by the user, we will apply some default headers.
type WebHook struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

const (
	// Is operator looks for exact pattern in a search query, i.e.
	// whether a search term "is" equal to condition.pattern.
	Is Operator = iota

	// Contains operator looks for the pattern in a search query, i.e.
	// whether a search term "contains" condition.pattern.
	Contains

	// StartsWith operator looks for the pattern in a search query as a prefix, i.e.
	// whether a search term "starts_with" condition.pattern.
	StartsWith

	// EndsWith operator looks for the pattern in a search query as a suffix, i.e.
	// whether a search term "ends_with" condition.pattern.
	EndsWith
)

// String is the implementation of Stringer interface that returns the string representation of query.Operator type.
func (o Operator) String() string {
	return [...]string{
		"is",
		"contains",
		"starts_with",
		"ends_with",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling query.Operator type.
func (o *Operator) UnmarshalJSON(bytes []byte) error {
	var operator string
	err := json.Unmarshal(bytes, &operator)
	if err != nil {
		return err
	}
	switch operator {
	case Is.String():
		*o = Is
	case Contains.String():
		*o = Contains
	case StartsWith.String():
		*o = StartsWith
	case EndsWith.String():
		*o = EndsWith
	default:
		return fmt.Errorf("invalid condition operator encountered: %v", operator)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling query.Operator type.
func (o Operator) MarshalJSON() ([]byte, error) {
	var operator string
	switch o {
	case Is:
		operator = Is.String()
	case Contains:
		operator = Contains.String()
	case StartsWith:
		operator = StartsWith.String()
	case EndsWith:
		operator = EndsWith.String()
	default:
		return nil, fmt.Errorf("invalid condition operator encountered: %v", o)
	}
	return json.Marshal(operator)
}
