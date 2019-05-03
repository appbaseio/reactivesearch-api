package query

// Then represents the action which is to be performed,
// when the rule condition is fulfilled by a search request.
type Then struct {
	Promote []interface{} `json:"promote,omitempty"`
	Hide    []struct {
		DocID *string `json:"doc_id"`
	} `json:"hide,omitempty"`
	WebHook *WebHook `json:"webhook,omitempty"`
}

// WebHook will contain information about the webhook which has to be called.
type WebHook struct {
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	PayloadTemplate interface{}       `json:"payload_template"`
}
