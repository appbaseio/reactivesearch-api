package openai

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// AISessionDoc will contain the session details that will
// be stored in ES
type AISessionDoc struct {
	SessionId          *string                   `json:"session_id"`
	UserId             *string                   `json:"user_id"`
	Useful             *bool                     `json:"useful"`
	Reason             *string                   `json:"reason,omitempty"`
	InputTokens        *int64                    `json:"input_tokens"`
	OutputTokens       *int64                    `json:"output_tokens"`
	InvocationCount    *int                      `json:"invocation_count"`
	Messages           *[]ChatGPTMessage         `json:"messages"`
	Model              *string                   `json:"model"`
	Index              []string                  `json:"index"`
	DocumentIds        []string                  `json:"document_ids"`
	CreatedAt          *int64                    `json:"created_at,omitempty"`
	UpdatedAt          *int64                    `json:"updated_at,omitempty"`
	TimeStamp          *int64                    `json:"timestamp"`
	Meta               *map[string]interface{}   `json:"meta,omitempty"`
	ResponseResolution ResponseResolutionDetails `json:"response_resolution,omitempty"`
	InternalResponse   *string                   `json:"internal_response,omitempty"`
}

type ResponseResolutionDetails struct {
	ResolvedAt           int64  `json:"resolved_at"`
	FirstByteReadAt      int64  `json:"first_byte_read_at"`
	ClosedAt             int64  `json:"closed_at"`
	CallReceivedAt       int64  `json:"call_received_at"`
	CallStreamResolvedAt int64  `json:"call_stream_resolved_at"`
	CallFirstWrittenAt   int64  `json:"first_written_at"`
	AICallCurl           string `json:"ai_call_curl"`
	AIBody               string `json:"ai_body"`
}

// UsefulAnalytics struct
type UsefulAnalytics struct {
	IsUseful *bool                   `json:"useful"`
	Reason   *string                 `json:"reason,omitempty"`
	UserID   *string                 `json:"user_id,omitempty"`
	Meta     *map[string]interface{} `json:"meta,omitempty"`
}

// FilterQueryParams will contain the query params allowed
// for filtering the sessions
type FilterQueryParams struct {
	FromTimeStamp *int64
	ToTimeStamp   *int64
	InvocationMin *int
	InvocationMax *int
	Useful        *bool
	Size          *int
	Offset        *int
	ModelPrefix   *[]string
	UserId        *[]string
}

// parseRangeParams will parse the range params as passed by the
// user.
//
// This function will return the following values:
// - from
// - to
// - size
func parseRangeParams(values url.Values) (int64, int64, int) {
	to := time.Now().Unix()
	from := time.Now().Add(-30 * 24 * time.Hour).Unix()
	size := 10000

	fromValue := values.Get("from_timestamp")
	if fromValue != "" {
		fromAsInt, convertErr := strconv.ParseInt(fromValue, 10, 64)
		if convertErr == nil {
			from = fromAsInt
		}
	}

	toValue := values.Get("to_timestamp")
	if toValue != "" {
		toAsInt, convertErr := strconv.ParseInt(toValue, 10, 64)
		if convertErr == nil {
			to = toAsInt
		}
	}

	sizePassed := values.Get("size")
	if sizePassed != "" {
		sizeAsInt, asIntErr := strconv.Atoi(sizePassed)
		if asIntErr == nil {
			if sizeAsInt < 10000 {
				size = sizeAsInt
			} else {
				log.Warnln(logTag, ": size exceeds allowed value of 10000, defaulting to 100!")
			}
		}
	}

	return from, to, size
}

// parseFilterParams will parse the filter params as passed by the
// user.
func parseFilterParams(values url.Values) FilterQueryParams {
	defaultSize := 30
	defaultOffset := 0
	queryParams := FilterQueryParams{Size: &defaultSize, Offset: &defaultOffset}

	invocationMinValue := values.Get("invocation_min")
	if invocationMinValue != "" {
		// Convert to int and save accordingly
		invocationMin, convertErr := strconv.Atoi(invocationMinValue)
		if convertErr == nil {
			queryParams.InvocationMin = &invocationMin
		}
	}

	invocationMaxValue := values.Get("invocation_max")
	if invocationMaxValue != "" {
		// Convert to int and save accordingly
		invocationMax, convertErr := strconv.Atoi(invocationMaxValue)
		if convertErr == nil {
			queryParams.InvocationMax = &invocationMax
		}
	}

	usefulValue := values.Get("useful")
	if usefulValue != "" {
		usefulBool := true
		if usefulValue == "false" {
			usefulBool = false
		}
		queryParams.Useful = &usefulBool
	}

	sizeValue := values.Get("size")
	if sizeValue != "" {
		sizeAsInt, convertErr := strconv.Atoi(sizeValue)
		if convertErr == nil && sizeAsInt <= 100 {
			queryParams.Size = &sizeAsInt
		}
	}

	offsetValue := values.Get("offset")
	if offsetValue != "" {
		offsetAsInt, convertErr := strconv.Atoi(offsetValue)
		if convertErr == nil {
			queryParams.Offset = &offsetAsInt
		}
	}

	// Support multiple values for the model_prefix field
	modelPrefixValues := values["model_prefix"]
	if len(modelPrefixValues) > 0 {
		queryParams.ModelPrefix = &modelPrefixValues
	}

	// Support multiple values for the user_id field
	userIdValues := values["user_id"]
	if len(userIdValues) > 0 {
		queryParams.UserId = &userIdValues
	}

	// Parse from_timestamp
	fromTimestampValue := values.Get("from_timestamp")
	if fromTimestampValue != "" {
		// Convert into int64
		fromTimestampConverted, convertErr := strconv.ParseInt(fromTimestampValue, 10, 64)
		if convertErr == nil {
			queryParams.FromTimeStamp = &fromTimestampConverted
		}
	}

	// Parse to_timestamp
	toTimestampValue := values.Get("to_timestamp")
	if toTimestampValue != "" {
		// Convert into int64
		toTimestampConverted, convertErr := strconv.ParseInt(toTimestampValue, 10, 64)
		if convertErr == nil {
			queryParams.ToTimeStamp = &toTimestampConverted
		}
	}

	return queryParams
}

// extractBucketsFromResponse will extract the buckets from the
// passed response by using the passed name of aggregation and then
// build a response that can be returned.
func extractBucketsFromResponse(result *es7.SearchResult, aggrName string) ([]map[string]interface{}, error) {
	aggrResult, found := result.Aggregations.Terms(aggrName)
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from '%s'", aggrName)
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount

		if bucket.KeyAsString != nil {
			newBucket["key_as_string"] = *bucket.KeyAsString
		}

		buckets = append(buckets, newBucket)
	}

	return buckets, nil
}

// extractSumFromResponse will extract the sum aggregation value from response
func extractSumFromResponse(result *es7.SearchResult, aggrName string) (float64, error) {
	aggrResult, found := result.Aggregations.Sum(aggrName)
	if !found {
		return 0, fmt.Errorf("unable to fetch aggregation value from '%s'", aggrName)
	}

	return *aggrResult.Value, nil
}

// extractSessionHistogram will take care of extracting the session histogram
// data from the result and accordingly return it.
func extractSessionHistogram(result *es7.SearchResult, aggrName string, subAggr string) ([]map[string]interface{}, error) {
	aggrResult, found := result.Aggregations.Terms(aggrName)
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from '%s'", aggrName)
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount

		if bucket.KeyAsString != nil {
			newBucket["key_as_string"] = *bucket.KeyAsString
		}

		// Extract the nested total_invocations
		invocationAggr, isFound := bucket.Aggregations.Sum(subAggr)
		if isFound {
			newBucket[subAggr] = *invocationAggr.Value
		}

		buckets = append(buckets, newBucket)
	}

	return buckets, nil
}
