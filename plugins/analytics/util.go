package analytics

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/antonmedv/expr"
	"github.com/appbaseio-confidential/reactivesearch/model/index"
	"github.com/appbaseio-confidential/reactivesearch/plugins/analyticsrequest"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/appbaseio-confidential/reactivesearch/util/iplookup"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const (
	defaultResponseSize = 100
	defaultTimeFormat   = "2006/01/02"
)

type ClickType int

const (
	Result ClickType = iota
	Suggestion
)

type SavedSearchRequest struct {
	QueryId         *string                 `json:"query_id,omitempty"`
	SavedSearchId   *string                 `json:"save_search_id,omitempty"`
	SavedSearchName *string                 `json:"save_search_name,omitempty"`
	SavedSearchMeta *map[string]interface{} `json:"save_search_meta,omitempty"`
	UserId          *string                 `json:"user_id,omitempty"`
	CustomEvents    *map[string]interface{} `json:"custom_events,omitempty"`
}

type FavoriteRequest struct {
	Id           *string                 `json:"id,omitempty"`
	QueryId      *string                 `json:"query_id,omitempty"`
	FavoriteOn   *string                 `json:"favorite_on,omitempty"`
	Source       *map[string]interface{} `json:"source,omitempty"`
	UserId       *string                 `json:"user_id,omitempty"`
	CustomEvents *map[string]interface{} `json:"custom_events,omitempty"`
	Meta         *map[string]interface{} `json:"meta,omitempty"`
}
type FavoriteES struct {
	Id                     *string                 `json:"id,omitempty"`
	QueryId                *string                 `json:"query_id,omitempty"`
	FavoriteOn             *string                 `json:"favorite_on,omitempty"`
	Source                 *string                 `json:"source,omitempty"`
	UserId                 *string                 `json:"user_id,omitempty"`
	CustomEvents           *map[string]interface{} `json:"custom_events,omitempty"`
	CreatedAt              *int64                  `json:"created_at,omitempty"`
	UpdatedAt              *int64                  `json:"updated_at,omitempty"`
	TimeStamp              *string                 `json:"timestamp,omitempty"`
	SearchQuery            *string                 `json:"search_query,omitempty"`
	SearchQueryCharsLength *int                    `json:"search_characters_length,omitempty"`
	Meta                   *map[string]interface{} `json:"meta,omitempty"`
}

type SavedSearchES struct {
	QueryId                *string                 `json:"query_id,omitempty"`
	SavedSearchId          *string                 `json:"save_search_id,omitempty"`
	SavedSearchName        *string                 `json:"save_search_name,omitempty"`
	SavedSearchMeta        *map[string]interface{} `json:"save_search_meta,omitempty"`
	UserId                 *string                 `json:"user_id,omitempty"`
	CustomEvents           *map[string]interface{} `json:"custom_events,omitempty"`
	SearchState            querytranslate.Endpoint `json:"search_state,omitempty"`
	CreatedAt              *int64                  `json:"created_at,omitempty"`
	UpdatedAt              *int64                  `json:"updated_at,omitempty"`
	TimeStamp              *string                 `json:"timestamp,omitempty"`
	SearchQuery            *string                 `json:"search_query,omitempty"`
	SearchQueryCharsLength *int                    `json:"search_characters_length,omitempty"`
}

// UserSession represents the user session record in the Elasticsearch
type UserSession struct {
	StartTime           *int64    `json:"start_time,omitempty"`
	LastInteractionTime *int64    `json:"last_interaction_time,omitempty"`
	Duration            *int64    `json:"duration,omitempty"`
	UserID              *string   `json:"user_id,omitempty"`
	Indices             *[]string `json:"indices,omitempty"`
	Bounce              *bool     `json:"bounce,omitempty"`
	TimeStamp           string    `json:"timestamp,omitempty"`
	IP                  *string   `json:"ip,omitempty"`
}

type UpdateConfig struct {
	DocID        string
	Script       string
	Record       map[string]interface{}
	ScriptParams map[string]interface{}
}

type ClickSummary struct {
	AvgClickPosition float64
	TotalConversions float64
	TotalClicks      float64
}

type ActiveUserSessionES struct {
	ID          string
	UserSession UserSession
}

type RequestHIT struct {
	ID    string `json:"id"`
	Index string `json:"index"`
}
type SearchRecord struct {
	Query       *string            `json:"query"`                 // Required
	QueryID     string             `json:"query_id"`              // Optional
	Filters     *map[string]string `json:"filters,omitempty"`     // Optional
	Impressions *[]RequestHIT      `json:"impressions,omitempty"` // Optional
	TotalHits   *int64             `json:"total_hits,omitempty"`  // Optional
	UserID      string             `json:"user_id,omitempty"`     // Optional
	// TODO: Remove `event_data` in Next major version
	EventData    map[string]interface{}  `json:"event_data,omitempty"`    // Optional
	CustomEvents map[string]interface{}  `json:"custom_events,omitempty"` // Optional
	Meta         *map[string]interface{} `json:"meta"`
}

type ClickRecord struct {
	Query     *string         `json:"query"`             // Optional
	QueryID   string          `json:"query_id"`          // Optional
	ClickType ClickType       `json:"click_type"`        // Optional
	ClickOn   *map[string]int `json:"click_on"`          // Required
	UserID    string          `json:"user_id,omitempty"` // Optional
	// TODO: Remove `event_data` in Next major version
	EventData    map[string]interface{}  `json:"event_data,omitempty"`    // Optional
	CustomEvents map[string]interface{}  `json:"custom_events,omitempty"` // Optional
	Meta         *map[string]interface{} `json:"meta"`
}

type ConversionRecord struct {
	QueryID      string                  `json:"query_id"`      // Optional
	ConversionOn *[]string               `json:"conversion_on"` // Required
	Meta         *map[string]interface{} `json:"meta"`
}

type User struct {
	Email string `json:"email"`
}

// SummaryRecord represents the summary record
type SummaryRecord struct {
	AvgBounceRate               float64 `json:"avg_bounce_rate"`
	AvgUserSessionDuration      float64 `json:"avg_user_session_duration"`
	AvgClickRate                float64 `json:"avg_click_rate"`
	AvgClickPosition            float64 `json:"avg_click_position"`
	AvgResultClickPosition      float64 `json:"avg_results_click_position"`
	AvgSuggestionsClickPosition float64 `json:"avg_suggestions_click_position"`
	AvgSuggestionsClickRate     float64 `json:"avg_suggestions_click_rate"`
	AvgConversionRate           float64 `json:"avg_conversion_rate"`
	TotalResultsCount           float64 `json:"total_results_count"`
	TotalSearches               float64 `json:"total_searches"`
	TotalUsers                  float64 `json:"total_users"`
	TotalUserSessions           float64 `json:"total_user_sessions"`
	TotalBounceUsers            float64 `json:"total_bounce_users"`
	TotalClicks                 float64 `json:"total_clicks"`
	TotalSuggestionsClicks      float64 `json:"total_suggestions_clicks"`
	TotalResultsClicks          float64 `json:"total_results_clicks"`
	TotalConversions            float64 `json:"total_conversions"`
	TotalNoResultsSearches      float64 `json:"total_no_results_searches"`
	NoResultsRate               float64 `json:"no_results_rate"`
}

// DetailedSummary represents the detailed summary to extract the insights
type DetailedSummary struct {
	AvgBounceRate               float64 `json:"avg_bounce_rate"`
	AvgUserSessionDuration      float64 `json:"avg_user_session_duration"`
	AvgClickRate                float64 `json:"avg_click_rate"`
	AvgClickPosition            float64 `json:"avg_click_position"`
	AvgResultClickPosition      float64 `json:"avg_results_click_position"`
	AvgSuggestionsClickPosition float64 `json:"avg_suggestions_click_position"`
	AvgSuggestionsClickRate     float64 `json:"avg_suggestions_click_rate"`
	AvgConversionRate           float64 `json:"avg_conversion_rate"`
	TotalResultsCount           float64 `json:"total_results_count"`
	TotalSearches               float64 `json:"total_searches"`
	TotalUsers                  float64 `json:"total_users"`
	TotalUserSessions           float64 `json:"total_user_sessions"`
	TotalBounceUsers            float64 `json:"total_bounce_users"`
	TotalClicks                 float64 `json:"total_clicks"`
	TotalSuggestionsClicks      float64 `json:"total_suggestions_clicks"`
	TotalResultsClicks          float64 `json:"total_results_clicks"`
	TotalConversions            float64 `json:"total_conversions"`
	TotalNoResultsSearches      float64 `json:"total_no_results_searches"`
	NoResultsRate               float64 `json:"no_results_rate"`
	PopularSearches             float64 `json:"popular_searches"`
	AvgClickRatePopularSearches float64 `json:"avg_click_rate_popular_searches"`
	AvgQueryLength              float64 `json:"avg_query_length"`
	InternalServerErrors        float64 `json:"internal_server_errors"`
	BadRequestErrors            float64 `json:"bad_request_errors"`
	HighResponseTimeSearches    float64 `json:"high_response_time_searches"`
}

// InsightEnvironments represents the environment variables for Insight Title
type InsightEnvironments struct {
	AvgBounceRate               string
	AvgUserSessionDuration      string
	AvgClickRate                string
	AvgClickPosition            string
	AvgResultClickPosition      string
	AvgSuggestionsClickPosition string
	AvgSuggestionsClickRate     string
	AvgConversionRate           string
	TotalResultsCount           string
	TotalSearches               string
	TotalUsers                  string
	TotalUserSessions           string
	TotalBounceUsers            string
	TotalClicks                 string
	TotalSuggestionsClicks      string
	TotalResultsClicks          string
	TotalConversions            string
	TotalNoResultsSearches      string
	NoResultsRate               string
	PopularSearches             string
	AvgClickRatePopularSearches string
	AvgQueryLength              string
	InternalServerErrors        string
	BadRequestErrors            string
	HighResponseTimeSearches    string
	ToFloat                     func(string) float64
	ToString                    func(float64, int) string
	FormatNumber                func(float64) string
}

// SummaryResponse represents the summary endpoint response struct
type SummaryResponse struct {
	Summary          SummaryRecord  `json:"summary"`
	CompateTimeframe *SummaryRecord `json:"compare_timeframe,omitempty"`
}

// GetInsight represents the returned shape for getInsights method
type GetInsight struct {
	Summary       SummaryRecord
	Insights      []InsightResponseType
	SavedInsights []InsightResponseType
	ReadInsights  []InsightResponseType
}

// SummaryComparison represents the analytics summary comparison environment variables available for mail template for analytics
type SummaryComparison struct {
	AvgBounceRateComparison               float64 `json:"avg_bounce_rate_sc"`
	AvgBounceRatePrevious                 float64 `json:"avg_bounce_rate_prev"`
	AvgUserSessionDurationComparison      float64 `json:"avg_user_session_duration_sc"`
	AvgUserSessionDurationPrevious        float64 `json:"avg_user_session_duration_prev"`
	AvgClickRateComparison                float64 `json:"avg_click_rate_sc"`
	AvgClickRatePrevious                  float64 `json:"avg_click_rate_prev"`
	AvgClickPositionComparison            float64 `json:"avg_click_position_sc"`
	AvgClickPositionPrevious              float64 `json:"avg_click_position_prev"`
	AvgResultClickPositionComparison      float64 `json:"avg_results_click_position_sc"`
	AvgResultClickPositionPrevious        float64 `json:"avg_results_click_position_prev"`
	AvgSuggestionsClickPositionComparison float64 `json:"avg_suggestions_click_position_sc"`
	AvgSuggestionsClickPositionPrevious   float64 `json:"avg_suggestions_click_position_prev"`
	AvgSuggestionsClickRateComparison     float64 `json:"avg_suggestions_click_rate_sc"`
	AvgSuggestionsClickRatePrevious       float64 `json:"avg_suggestions_click_rate_prev"`
	AvgConversionRateComparison           float64 `json:"avg_conversion_rate_sc"`
	AvgConversionRatePrevious             float64 `json:"avg_conversion_rate_prev"`
	TotalResultsCountComparison           float64 `json:"total_results_count_sc"`
	TotalResultsCountPrevious             float64 `json:"total_results_count_prev"`
	TotalSearchesComparison               float64 `json:"total_searches_sc"`
	TotalSearchesPrevious                 float64 `json:"total_searches_prev"`
	TotalUsersComparison                  float64 `json:"total_users_sc"`
	TotalUsersPrevious                    float64 `json:"total_users_prev"`
	TotalUserSessionsComparison           float64 `json:"total_user_sessions_sc"`
	TotalUserSessionsPrevious             float64 `json:"total_user_sessions_prev"`
	TotalBounceUsersComparison            float64 `json:"total_bounce_users_sc"`
	TotalBounceUsersPrevious              float64 `json:"total_bounce_users_prev"`
	TotalClicksComparison                 float64 `json:"total_clicks_sc"`
	TotalClicksPrevious                   float64 `json:"total_clicks_prev"`
	TotalSuggestionsClicksComparison      float64 `json:"total_suggestions_clicks_sc"`
	TotalSuggestionsClicksPrevious        float64 `json:"total_suggestions_clicks_prev"`
	TotalResultsClicksComparison          float64 `json:"total_results_clicks_sc"`
	TotalResultsClicksPrevious            float64 `json:"total_results_clicks_prev"`
	TotalConversionsComparison            float64 `json:"total_conversions_sc"`
	TotalConversionsPrevious              float64 `json:"total_conversions_prev"`
	TotalNoResultsSearchesComparison      float64 `json:"total_no_results_searches_sc"`
	TotalNoResultsSearchesPrevious        float64 `json:"total_no_results_searches_prev"`
	NoResultsRateComparison               float64 `json:"no_results_rate_sc"`
	NoResultsRatePrevious                 float64 `json:"no_results_rate_prev"`
}

// InsightReportAnalytics represents the insight object
type InsightReportAnalytics struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Link        string `json:"link"`
}

// InsightsEnvironments represents the insights environment variables available for mail template for analytics
type InsightsEnvironments struct {
	DateRange        string                   `json:"date_range"` // format Apr 1 - Apr 30
	NumberOfInsights int                      `json:"number_of_insights"`
	Insights         []InsightReportAnalytics `json:"insights"`
}

// ReportAnalyticsRequest represents the request body for report analytics
type ReportAnalyticsRequest struct {
	Summary             SummaryRecord        `json:"summary"`
	SummaryComparison   SummaryComparison    `json:"summary_comparison"`
	Insights            InsightsEnvironments `json:"insights"`
	Users               []string             `json:"users"`
	FeatureCustomEvents bool                 `json:"feature_custom_events"`
}

func getInsightEnvironmentsFromSummary(summary DetailedSummary) InsightEnvironments {
	return InsightEnvironments{
		AvgBounceRate:               strconv.FormatFloat(summary.AvgBounceRate, 'f', 0, 64),
		AvgUserSessionDuration:      strconv.FormatFloat(summary.AvgUserSessionDuration, 'f', 2, 64),
		AvgClickRate:                strconv.FormatFloat(summary.AvgClickRate, 'f', 2, 64),
		AvgSuggestionsClickRate:     strconv.FormatFloat(summary.AvgSuggestionsClickRate, 'f', 2, 64),
		AvgConversionRate:           strconv.FormatFloat(summary.AvgConversionRate, 'f', 2, 64),
		TotalResultsCount:           strconv.FormatFloat(summary.TotalResultsCount, 'f', 0, 64),
		TotalSearches:               strconv.FormatFloat(summary.TotalSearches, 'f', 0, 64),
		TotalUsers:                  strconv.FormatFloat(summary.TotalUsers, 'f', 0, 64),
		TotalUserSessions:           strconv.FormatFloat(summary.TotalUserSessions, 'f', 0, 64),
		TotalBounceUsers:            strconv.FormatFloat(summary.TotalBounceUsers, 'f', 0, 64),
		TotalClicks:                 strconv.FormatFloat(summary.TotalClicks, 'f', 0, 64),
		TotalSuggestionsClicks:      strconv.FormatFloat(summary.TotalSuggestionsClicks, 'f', 0, 64),
		TotalResultsClicks:          strconv.FormatFloat(summary.TotalResultsClicks, 'f', 0, 64),
		TotalConversions:            strconv.FormatFloat(summary.TotalConversions, 'f', 0, 64),
		TotalNoResultsSearches:      strconv.FormatFloat(summary.TotalNoResultsSearches, 'f', 0, 64),
		NoResultsRate:               strconv.FormatFloat(summary.NoResultsRate, 'f', 2, 64),
		PopularSearches:             strconv.FormatFloat(summary.PopularSearches, 'f', 0, 64),
		AvgClickRatePopularSearches: strconv.FormatFloat(summary.AvgClickRatePopularSearches, 'f', 2, 64),
		AvgQueryLength:              strconv.FormatFloat(summary.AvgQueryLength, 'f', 2, 64),
		InternalServerErrors:        strconv.FormatFloat(summary.InternalServerErrors, 'f', 0, 64),
		BadRequestErrors:            strconv.FormatFloat(summary.BadRequestErrors, 'f', 0, 64),
		HighResponseTimeSearches:    strconv.FormatFloat(summary.HighResponseTimeSearches, 'f', 2, 64),
		ToFloat:                     toFloat,
		ToString:                    toString,
		FormatNumber:                formatNumber,
	}
}

type AnalyticsPreferences struct {
	Enable *bool `json:"enable"`
}

func evaluateStringExpression(value string, name string, insightID string, envs InsightEnvironments) (string, error) {
	errorMsg := "error encountered while validating " + name + " expression for " + insightID + " type"
	program, err := expr.Compile(value, expr.Env(envs))
	if err != nil {
		return "", fmt.Errorf(errorMsg, err)
	}

	output, err := expr.Run(program, envs)
	if err != nil {
		return "", fmt.Errorf(errorMsg, err)
	}
	evaluatedExpr, ok := output.(string)
	if !ok {
		return "", fmt.Errorf(errorMsg, err)
	}
	return evaluatedExpr, nil
}

// parse a string to float
func toFloat(value string) float64 {
	if s, err := strconv.ParseFloat(value, 64); err == nil {
		return s
	}
	return 0
}

// parse a float to string
func toString(value float64, prec int) string {
	return strconv.FormatFloat(value, 'f', prec, 64)
}

// Formats the float value to comma separated string
func formatNumber(value float64) string {
	p := message.NewPrinter(language.English)
	return p.Sprintf("%d", int(value))
}

// To check if an item is present in a slice
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// To check if a insight status is present in a slice
func isInsightExist(s []InsightType, e InsightType) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// String is the implementation of Stringer interface that returns the string representation of ClickType type.
func (o ClickType) String() string {
	return [...]string{
		"result",
		"suggestion",
	}[o]
}

// UnmarshalJSON is the implementation of the Unmarshaler interface for un-marshaling ClickType type.
func (o *ClickType) UnmarshalJSON(bytes []byte) error {
	var clicktype string
	err := json.Unmarshal(bytes, &clicktype)
	if err != nil {
		return err
	}
	switch clicktype {
	case Result.String():
		*o = Result
	case Suggestion.String():
		*o = Suggestion
	default:
		return fmt.Errorf("invalid clicktype encountered: %v", clicktype)
	}
	return nil
}

// MarshalJSON is the implementation of the Marshaler interface for marshaling ClickType type.
func (o ClickType) MarshalJSON() ([]byte, error) {
	var clicktype string
	switch o {
	case Result:
		clicktype = Result.String()
	case Suggestion:
		clicktype = Suggestion.String()
	default:
		return nil, fmt.Errorf("invalid clicktype encountered: %v", o)
	}
	return json.Marshal(clicktype)
}

// All query params will be considered as the custom user filters except the below filters.
var preDefinedFilters = [...]string{"from", "size", "to", "click_analytics", "min_chars", "query", "show_global", "from_timestamp", "to_timestamp", "time_zone"}

// Need to define separately so we can decide whether to apply custom event prefix in field or not while querying ES.
var PreDefinedTermFilters = [...]string{"user_id", "ip"}

func getESRecord(record analyticsrequest.Record) map[string]interface{} {
	var finalRecord map[string]interface{}
	marshalled, _ := json.Marshal(record)
	// TODO: Optimize it => can use different marshaller
	json.Unmarshal(marshalled, &finalRecord)
	if finalRecord["custom_events"] != nil {
		if customEvents, ok := finalRecord["custom_events"].([]interface{}); ok {
			if len(customEvents) > 0 {
				for _, element := range customEvents {
					if parsedValue, ok := element.(map[string]interface{}); ok {
						if key, ok2 := parsedValue["key"].(string); ok2 {
							finalRecord[key] = parsedValue["value"]
						}
					}
				}
			}
		}
	}
	delete(finalRecord, "custom_events")
	return finalRecord
}

func isCustomFilter(e string) bool {
	for _, a := range preDefinedFilters {
		if a == e {
			return false
		}
	}
	return true
}

func IsPreDefinedTermFilter(e string) bool {
	for _, a := range PreDefinedTermFilters {
		if a == e {
			return true
		}
	}
	return false
}

func getCustomFilters(values url.Values) map[string]interface{} {
	filters := make(map[string]interface{})
	for filter := range values {
		if isCustomFilter(filter) {
			filters[filter] = values.Get(filter)
		}
	}
	return filters
}

// GetCustomFilters will parse the custom filters from the query
// params map
func GetCustomFilters(values url.Values) map[string]interface{} {
	return getCustomFilters(values)
}

type QueryParams struct {
	From     string
	To       string
	Size     int
	Timezone string
}

// TODO: Make this function to return a map with default values for required query params?
// rangeQueryParams returns the common query params that every analytics endpoint expects,
// - "from": start of the duration in consideration
// - "to"  : end of the duration in consideration
// - "size": no. of response entries
func rangeQueryParams(values url.Values) QueryParams {
	from, to := previousMonthRange()
	size := 100

	// priority is given to the unix timestamp in seconds
	// if not able to parse then it'll try to parse to `2006/01/02` format (deprecated)
	// if both parsing fails then it'll assign the default range to the current month

	fromTimestamp := values.Get("from_timestamp")
	fromValue := values.Get("from")
	if fromTimestamp != "" {
		fromTimestampAsInt, err := strconv.Atoi(fromTimestamp)
		if err != nil {
			log.Errorln(logTag, `: invalid "from_timestamp" value provided, defaulting to previous month:`, err)
		} else {
			from = time.Unix(int64(fromTimestampAsInt), 0).UTC().Format(time.RFC3339)
		}
	} else if fromValue != "" {
		t, err := time.Parse(defaultTimeFormat, fromValue)
		if err != nil {
			log.Errorln(logTag, `: unsupported "from" value provided, defaulting to previous month:`, err)
		} else {
			from = t.UTC().Format(time.RFC3339)
		}
	}

	toTimestamp := values.Get("to_timestamp")
	toValue := values.Get("to")

	if toTimestamp != "" {
		toTimestampAsInt, err := strconv.Atoi(toTimestamp)
		if err != nil {
			log.Errorln(logTag, `: invalid "to_timestamp" value provided, defaulting to current time:`, err)
		} else {
			to = time.Unix(int64(toTimestampAsInt), 0).UTC().Format(time.RFC3339)
		}
	} else if toValue != "" {
		t, err := time.Parse(defaultTimeFormat, toValue)
		if err != nil {
			log.Errorln(logTag, `: unsupported "to" value provided, defaulting to current time:`, err)
		} else {
			// Use end of the day for to range
			year, month, day := t.Date()
			to = time.Date(year, month, day, 23, 59, 59, 0, t.Location()).UTC().Format(time.RFC3339)
		}
	}

	respSize := values.Get("size")
	if respSize != "" {
		value, err := strconv.Atoi(respSize)
		if err != nil {
			value = defaultResponseSize
			log.Errorln(logTag, `: invalid "size" value provided, defaulting to 100:`, err)
		}
		if value > 1000 {
			value = defaultResponseSize
			log.Println(logTag, `: "size" limit exceeded (> 1000), default to 100`)
		}
		size = value
	}

	timezone := values.Get("time_zone")

	return QueryParams{
		From:     from,
		To:       to,
		Size:     size,
		Timezone: timezone,
	}
}

// RangeQueryParams exposes the rangeQueryParams method to make it accessible in the whole
// package.
func RangeQueryParams(values url.Values) QueryParams {
	return rangeQueryParams(values)
}

// previousMonthRange returns one month's duration starting from the current instant 30 days ago.
func previousMonthRange() (from, to string) {
	now := time.Now()
	from = now.AddDate(0, 0, -30).Format(time.RFC3339)
	to = now.Format(time.RFC3339)
	return
}

// parse splits the comma separated key-value pairs (k1=v1, k2=v3) present in the header.
func parse(header string) []querytranslate.TermFilter {
	var m []querytranslate.TermFilter
	tokens := strings.Split(header, ",")
	for _, token := range tokens {
		values := strings.Split(token, "=")
		if len(values) == 2 {
			// Use lower case for filter key and value
			m = append(m, querytranslate.TermFilter{
				Key:   strings.TrimSpace(values[0]),
				Value: strings.ToLower(strings.TrimSpace(values[1])),
			})
		}
	}
	return m
}

// TODO: we can use enums for query types
type QueryFilter struct {
	Type       string // Valid types are `exists` => We can add new types as per need
	Field      string
	NestedPath string
}

// parse splits the comma separated key-value pairs (k1=v1, k2=v3) present in the header.
func parseCustomEvents(header string) []analyticsrequest.CustomEvent {
	var m []analyticsrequest.CustomEvent
	tokens := strings.Split(header, ",")
	for _, token := range tokens {
		values := strings.Split(token, "=")
		if len(values) == 2 {
			event := analyticsrequest.CustomEvent{}
			event.Key = CustomEventsPrefix + strings.TrimSpace(values[0])
			event.Value = strings.TrimSpace(values[1])
			m = append(m, event)
		}
	}
	return m
}

// Common utility to extract the analytics values from headers
func calculateAnalytics(r *http.Request, record *analyticsrequest.Record) analyticsrequest.Record {
	// Collect search filters
	searchFilters := parse(r.Header.Get(XSearchFilters))
	if len(searchFilters) > 0 {
		record.SearchFilters = &searchFilters
	}
	// To record search state
	searchState := r.Header.Get(XSearchState)
	if searchState != "" {
		record.SearchState = searchState
	}

	// To record user id
	userID := r.Header.Get(XUserID)
	if userID != "" {
		record.UserID = userID
	} else {
		record.UserID = record.IP
	}

	clickObjectID := r.Header.Get(XSearchClickObjectID)
	// Search click
	if clickObjectID != "" {
		searchClick := r.Header.Get(XSearchClick)
		if searchClick != "" {
			if clicked, err := strconv.ParseBool(searchClick); err == nil {
				if clicked {
					record.ResultClickObjectIds = []string{clickObjectID}
				}
			} else {
				log.Errorln(logTag, ": invalid bool value", searchClick, "passed for header", XSearchClick, ":", err)
			}
		}
		// Search suggestions click
		searchSuggestionsClick := r.Header.Get(XSearchSuggestionsClick)
		if searchSuggestionsClick != "" {
			if clicked, err := strconv.ParseBool(searchSuggestionsClick); err == nil {
				// clicks includes both suggestions & results clicks
				if clicked {
					record.SuggestionsClickObjectIds = []string{clickObjectID}
				}
			} else {
				log.Errorln(logTag, ": invalid bool value", searchSuggestionsClick, "passed for header", XSearchSuggestionsClick, ":", err)
			}
		}
		// Click position
		searchClickPosition := r.Header.Get(XSearchClickPosition)
		if searchClickPosition != "" {
			if pos, err := strconv.Atoi(searchClickPosition); err == nil {
				record.ResultClickPositionIds = []int{pos}
			} else {
				log.Errorln(logTag, ": invalid int value ", searchClickPosition, " passed for header", XSearchClickPosition, ":", err)
			}
		}

		// SuggestionsClick position
		searchSuggestionsClickPosition := r.Header.Get(XSearchSuggestionsClickPosition)
		if searchSuggestionsClickPosition != "" {
			if pos, err := strconv.Atoi(searchSuggestionsClickPosition); err == nil {
				record.SuggestionsClickPositionIds = []int{pos}
			} else {
				log.Errorln(logTag, ": invalid int value", searchSuggestionsClickPosition, " passed for header", XSearchSuggestionsClickPosition, ":", err)
			}
		}
	}

	// conversion
	searchConversion := r.Header.Get(XSearchConversion)
	if searchConversion != "" {
		if conversion, err := strconv.ParseBool(searchConversion); err == nil {
			if conversion {
				record.ConversionObjectIds = []string{"null"}
			}
		} else {
			log.Errorln(logTag, ": invalid bool value", searchConversion, "passed for header", XSearchConversion, ":", err)
		}
	}

	customEvents := parseCustomEvents(r.Header.Get(XSearchCustomEvent))
	record.CustomEvents = customEvents
	return *record
}

func calculateAnalyticsForRSAPI(rsRequestBody *querytranslate.RSQuery, record *analyticsrequest.Record) analyticsrequest.Record {
	// Common utility to extract the analytics values from RS request body
	if rsRequestBody == nil {
		return *record
	}
	envs := querytranslate.ExtractEnvsFromRequest(*rsRequestBody)

	// Collect search filters
	if len(envs.TermFilters) > 0 {
		record.SearchFilters = &envs.TermFilters
	}

	// To record search state
	searchState, err := json.Marshal(rsRequestBody)
	if err != nil {
		log.Errorln(logTag, "error encountered while recording search state", err)
	}
	record.SearchState = string(searchState)
	// To record user id
	if rsRequestBody.Settings != nil && rsRequestBody.Settings.UserID != nil {
		record.UserID = *rsRequestBody.Settings.UserID
	} else {
		record.UserID = record.IP
	}

	// To record custom events
	if rsRequestBody.Settings != nil && rsRequestBody.Settings.CustomEvents != nil {
		var customEvents []analyticsrequest.CustomEvent
		for k, v := range *rsRequestBody.Settings.CustomEvents {
			customEvents = append(customEvents, analyticsrequest.CustomEvent{
				Key:   CustomEventsPrefix + k,
				Value: v,
			})
		}
		record.CustomEvents = customEvents
	}
	return *record
}

func generateSessionKey(userID, query string) string {
	return userID + separator + query
}

// Removes the prefix from custom event field
func trimEventPrefix(key string) string {
	return strings.TrimPrefix(key, CustomEventsPrefix)
}

// Adds the prefix from custom event field
func AddEventPrefix(key string) string {
	return CustomEventsPrefix + key
}

func extractIPFromRequest(r *http.Request) string {
	return iplookup.FromRequest(r)
}

type Location struct {
	Country     string
	City        string
	Coordinates string
}

func extractLocationFromRequest(r *http.Request) Location {
	ipInfo := iplookup.Instance()
	ipAddr := extractIPFromRequest(r)
	location := Location{}
	ipLookup, err := ipInfo.Lookup(ipAddr)
	if err != nil {
		log.Errorln(logTag, ": error fetching location coordinates for ip=", ipAddr, ":", err)
		return location
	}
	location.Coordinates = fmt.Sprintf("%s, %s", ipLookup.Lat, ipLookup.Lon)
	location.Country = ipLookup.Country
	location.City = ipLookup.City
	return location
}

func extractCustomEvents(eventData map[string]interface{}) []analyticsrequest.CustomEvent {
	var customEvents []analyticsrequest.CustomEvent
	for key, event := range eventData {
		customEvents = append(customEvents, analyticsrequest.CustomEvent{
			Key:   CustomEventsPrefix + key,
			Value: event,
		})
	}
	return customEvents
}

func extractObjectIds(data map[string]int) []string {
	var objectIds []string
	for key := range data {
		objectIds = append(objectIds, key)
	}
	return objectIds
}

func extractClickPositions(data map[string]int) []int {
	var clickPositions []int
	for _, position := range data {
		clickPositions = append(clickPositions, position)
	}
	return clickPositions
}

func addMetaDataToNewRecord(userID string, record *analyticsrequest.Record, r *http.Request) (analyticsrequest.Record, error) {
	record.IP = extractIPFromRequest(r)
	record.TimeStamp = time.Now().Format(time.RFC3339)
	if userID == "" {
		record.UserID = record.IP
	} else {
		record.UserID = userID
	}
	ctxIndices, err := index.FromContext(r.Context())
	if err != nil {
		log.Errorln(logTag, ": cannot fetch indices from request context, ", err)
		return *record, err
	}
	record.Indices = ctxIndices
	location := extractLocationFromRequest(r)
	record.Location = location.Coordinates
	record.Country = location.Country
	record.City = location.City
	return *record, nil
}

func getInsightStatusID(indexName string) string {
	currentMonth := strings.ToLower(time.Now().UTC().Format("Jan-2006"))
	docID := indexName + currentMonth
	return docID
}

func getMonthRange(addMonths int) (from time.Time, to time.Time) {
	now := time.Now().AddDate(0, addMonths, 0)
	currentYear, currentMonth, _ := now.Date()
	currentLocation := now.Location()
	firstOfMonth := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, currentLocation)
	year, month, day := firstOfMonth.AddDate(0, 1, -1).Date()
	lastOfMonth := time.Date(year, month, day, 23, 59, 59, 0, time.Now().Location())
	return firstOfMonth, lastOfMonth
}

func calculateSummaryComparison(preValue float64, value float64) float64 {
	if preValue == 0 {
		// divide by zero exception
		return 0
	}
	return util.WithPrecision(((value-preValue)/preValue)*100, 2)
}

// Returns the insight redirection link to dashboard/arc-dashboard based on the billing
func getInsightsEmailLink(insightID string) string {
	appbaseDashboard := "https://dashboard.reactivesearch.io/"
	arcDashboard := "http://dash.reactivesearch.io/"
	suffixURL := "?insights-sidebar=true&insights-id=" + insightID
	// Use appbase dashboard for clusters and byoc
	if util.ClusterBilling == "true" || util.HostedBilling == "true" {
		return appbaseDashboard + "clusters/" + util.ClusterID + suffixURL
	}
	// Use arc dashboard for self-hosted
	return arcDashboard + "cluster/analytics" + suffixURL
}

func validateCustomEvents(customEvents map[string]interface{}) error {
	for k, v := range customEvents {
		_, ok := v.(string)
		if !ok {
			valueAsInterface, ok := v.([]interface{})
			if !ok {
				return errors.New("Custom event " + k + " value must be a string or an array of strings")
			}
			for _, v1 := range valueAsInterface {
				_, ok := v1.(string)
				if !ok {
					return errors.New("Custom event " + k + " value must be a string or an array of strings")
				}
			}
		}
	}
	return nil
}

func GetAnalyticsIndex() string {
	analyticsIndex := os.Getenv(envAnalyticsEsIndex)
	if analyticsIndex == "" {
		analyticsIndex = defaultAnalyticsEsIndex
	}
	return analyticsIndex
}

func GetDocumentSuggestionsIndex() string {
	recentDocumentsIndex := os.Getenv(envRecentSearchesEsIndex)
	if recentDocumentsIndex == "" {
		recentDocumentsIndex = defaultRecentSearchesEsIndex
	}
	return recentDocumentsIndex
}

func getQueryTypeByID(id string, request querytranslate.RSQuery) *querytranslate.QueryType {
	for _, query := range request.Query {
		if query.ID != nil && *query.ID == id {
			return &query.Type
		}
	}
	return nil
}

func getTotalCountSuggestions(suggestions []querytranslate.SuggestionHIT) int64 {
	var total int64
	for _, suggestion := range suggestions {
		// only count index and popular suggestions for analytics
		if suggestion.Type == querytranslate.Index ||
			suggestion.Type == querytranslate.Popular {
			total += 1
		}
	}
	return total
}
