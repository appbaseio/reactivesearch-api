package query

import (
	"encoding/json"
	"fmt"
)

// Then represents the action which is to be performed,
// when the rule condition is fulfilled by a search request.
type Then struct {
	Action   Action    `json:"action"`
	Payloads []Payload `json:"payload"`
}

// Payload represents the document IDs (and optionally documents) associated with a consequence.
type Payload struct {
	DocID string      `json:"doc_id,omitempty"`
	Doc   interface{} `json:"doc,omitempty"`
}

// Action represents the consequent actions that can be performed.
type Action int

const (
	// Promote operation promotes the payload documents alongside the search result.
	Promote Action = iota

	// Hide operation hides the payload documents present in the search result.
	Hide
)

// String is the implementation of Stringer interface that returns the string representation of query.Then type.
func (o Action) String() string {
	return [...]string{
		"promote",
		"hide",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for unmarshaling query.Then type.
func (o *Action) UnmarshalJSON(bytes []byte) error {
	var action string
	err := json.Unmarshal(bytes, &action)
	if err != nil {
		return err
	}
	switch action {
	case Promote.String():
		*o = Promote
	case Hide.String():
		*o = Hide
	default:
		return fmt.Errorf("invalid consequence action encountered: %v", action)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling query.Then type.
func (o Action) MarshalJSON() ([]byte, error) {
	var action string
	switch o {
	case Promote:
		action = Promote.String()
	case Hide:
		action = Hide.String()
	default:
		return nil, fmt.Errorf("invalid consequence action encountered: %v", o)
	}
	return json.Marshal(action)
}
