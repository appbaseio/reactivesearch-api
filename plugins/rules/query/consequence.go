package query

import (
	"encoding/json"
	"fmt"
)

// Consequence represents the action which is to be performed,
// when the rule condition is fulfilled by a search request.
type Consequence struct {
	Operation Operation `json:"operation"`
	Payload   []Payload `json:"payload"`
}

// Payload represents the document IDs (and optionally documents) sassociated with a consequence.
type Payload struct {
	DocID    string `json:"doc_id"`
	Doc      string `json:"doc,omitempty"`
	Position string `json:"position,omitempty"`
}

// Operation represents the consequent actions that can be performed.
type Operation int

const (
	// Promote operation promotes the payload documents alongside the search result.
	Promote Operation = iota

	// Hide operation hides the payload documents present in the search result.
	Hide

	// Inject operation injects a custom payload documents alongside the search result.
	Inject
)

// String is the implementation of Stringer interface that returns the string representation of query.Operation type.
func (o Operation) String() string {
	return [...]string{
		"promote",
		"hide",
		"inject",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling query.Operation type.
func (o *Operation) UnmarshalJSON(bytes []byte) error {
	var operator string
	err := json.Unmarshal(bytes, &operator)
	if err != nil {
		return err
	}
	switch operator {
	case Promote.String():
		*o = Promote
	case Hide.String():
		*o = Hide
	case Inject.String():
		*o = Inject
	default:
		return fmt.Errorf("invalid consequence operator encountered: %v", operator)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling query.Operation type.
func (o Operation) MarshalJSON() ([]byte, error) {
	var operator string
	switch o {
	case Promote:
		operator = Promote.String()
	case Hide:
		operator = Hide.String()
	case Inject:
		operator = Inject.String()
	default:
		return nil, fmt.Errorf("invalid consequence operator encountered: %v", o)
	}
	return json.Marshal(operator)
}
