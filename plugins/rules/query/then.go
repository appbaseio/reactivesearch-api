package query

// Then represents the action which is to be performed,
// when the rule condition is fulfilled by a search request.
type Then struct {
	Promote []interface{} `json:"promote,omitempty"`
	Hide    []struct {
		DocID *string `json:"doc_id"`
	} `json:"hide,omitempty"`
}
