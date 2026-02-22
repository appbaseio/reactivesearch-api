package analyticsrequest

import "github.com/appbaseio/reactivesearch-api/plugins/querytranslate"

type HIT struct {
	ID    string `json:"id"`
	Index string `json:"index"`
}

type CustomEvent struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// Record represents the analytics doc for a record
type Record struct {
	Took                        float64                      `json:"took,omitempty"`
	SearchQuery                 *string                      `json:"search_query,omitempty"`
	QueryLength                 *int                         `json:"search_characters_length,omitempty"`
	SearchQueryLength           *int                         `json:"search_query_length,omitempty"`
	Index                       string                       `json:"index,omitempty"`
	Indices                     []string                     `json:"indices,omitempty"`
	HitsInResponse              *[]HIT                       `json:"hits_in_response,omitempty"`
	TotalHits                   *int64                       `json:"total_hits,omitempty"`
	TimeStamp                   string                       `json:"timestamp,omitempty"`
	IP                          string                       `json:"ip,omitempty"`
	Location                    string                       `json:"location,omitempty"`
	Country                     string                       `json:"country,omitempty"`
	City                        string                       `json:"city,omitempty"`
	SearchFilters               *[]querytranslate.TermFilter `json:"search_filters,omitempty"`
	SearchState                 string                       `json:"search_state,omitempty"`
	UserID                      string                       `json:"user_id,omitempty"`
	ResultClickObjectIds        []string                     `json:"result_click_object_ids,omitempty"`
	ResultClickPositionIds      []int                        `json:"result_click_position_ids,omitempty"`
	SuggestionsClickObjectIds   []string                     `json:"suggestion_click_object_ids,omitempty"`
	SuggestionsClickPositionIds []int                        `json:"suggestion_click_position_ids,omitempty"`
	Conversion                  bool                         `json:"conversion,omitempty"`
	ConversionObjectIds         []string                     `json:"conversion_object_ids,omitempty"`
	StoredQueries               *[]string                    `json:"storedqueries,omitempty"`
	QueryRules                  *[]string                    `json:"queryrules,omitempty"`
	CustomEvents                []CustomEvent                `json:"custom_events,omitempty"` // Note: we save it differently
	URL                         *string                      `json:"url,omitempty"`
	Method                      *string                      `json:"method,omitempty"`
	Meta                        *map[string]interface{}      `json:"meta"`
}
