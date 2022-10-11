package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/difference"
	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) getRawLogsES7(ctx context.Context, logsFilter logsFilter) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(logsFilter.StartDate).
		To(logsFilter.EndDate)

	searchQuery := es7.NewBoolQuery().Filter(duration)
	// apply category filter
	if logsFilter.Filter == "search" {
		filters := es7.NewTermsQuery("category.keyword", []interface{}{"search", category.ReactiveSearch.String(), "suggestion"}...)
		searchQuery.Filter(filters)
	} else if logsFilter.Filter == "suggestion" {
		filters := es7.NewTermsQuery("category.keyword", []interface{}{"suggestion"}...)
		searchQuery.Filter(filters)
	} else if logsFilter.Filter == "index" {
		filters := []es7.Query{
			es7.NewTermsQuery("request.method.keyword", []interface{}{"POST", "PUT"}...),
			es7.NewTermsQuery("category.keyword", []interface{}{"docs"}...),
			es7.NewRangeQuery("response.code").Gte(200).Lte(299),
		}
		searchQuery.Filter(filters...)
	} else if logsFilter.Filter == "delete" {
		filters := es7.NewMatchQuery("request.method.keyword", "DELETE")
		searchQuery.Filter(filters)
	} else if logsFilter.Filter == "success" {
		filters := es7.NewRangeQuery("response.code").Gte(200).Lte(299)
		searchQuery.Filter(filters)
	} else if logsFilter.Filter == "error" {
		filters := es7.NewRangeQuery("response.code").Gte(400)
		searchQuery.Filter(filters)
	} else {
		searchQuery.Filter(es7.NewMatchAllQuery())
	}

	// apply index filtering logic
	util.GetIndexFilterQueryEs7(searchQuery, logsFilter.Indices...)

	// only apply latency filter when start or end range is available
	if logsFilter.StartLatency != nil || logsFilter.EndLatency != nil {
		latencyRangeQuery := es7.NewRangeQuery("response.took")
		if logsFilter.StartLatency != nil {
			latencyRangeQuery.Gte(*logsFilter.StartLatency)
		}
		if logsFilter.EndLatency != nil {
			latencyRangeQuery.Lte(*logsFilter.EndLatency)
		}
		searchQuery.Filter(latencyRangeQuery)
	}

	searchRequest := util.GetInternalClient7().
		Search(es.indexName).
		Query(searchQuery).
		From(logsFilter.Offset).
		Size(logsFilter.Size)

	if logsFilter.OrderByLatency != "" {
		ascending := false
		if logsFilter.OrderByLatency == "asc" {
			ascending = true
		}
		// sort by latency
		searchRequest.SortWithInfo(es7.SortInfo{Field: "response.took", UnmappedType: "int", Ascending: ascending})
	}
	searchRequest.SortWithInfo(es7.SortInfo{Field: "timestamp", UnmappedType: "date", Ascending: false})
	response, err := util.SearchRequestDo(searchRequest, searchQuery, context.Background())
	if err != nil {
		return nil, err
	}

	hits := make([]map[string]interface{}, 0)
	for _, hit := range response.Hits.Hits {
		var source map[string]interface{}
		err := json.Unmarshal(hit.Source, &source)
		if err != nil {
			return nil, err
		}

		// Extract the log ID
		source["id"] = hit.Id
		// Prase stringified headers
		source = ParseHeaderString(source, "headers_string", "header")
		hits = append(hits, source)
	}

	logs := make(map[string]interface{})
	logs["logs"] = hits
	logs["total"] = response.Hits.TotalHits.Value
	logs["took"] = response.TookInMillis

	raw, err := json.Marshal(logs)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// getRawLogES7 will get the raw log for the log with passed ID.
// If we don't find a match, we will raise a 404 error.
func (es *elasticsearch) getRawLogES7(ctx context.Context, ID string, parseDiffs bool) ([]byte, *LogError) {
	// Create the query
	searchQuery := es7.NewTermQuery("_id", ID)

	searchRequest := util.GetInternalClient7().Search().
		Index(es.indexName).Query(searchQuery).Size(1)

	response, err := util.SearchRequestDo(searchRequest, searchQuery, context.Background())

	if err != nil {
		errCode := http.StatusInternalServerError
		log.Errorln(logTag, ": error while getting log by ID")
		return nil, &LogError{
			Err:  err,
			Code: errCode,
		}
	}

	if len(response.Hits.Hits) == 0 {
		return nil, &LogError{
			Err:  errors.New(fmt.Sprintf("Log not found with ID: %s", ID)),
			Code: http.StatusNotFound,
		}
	}

	logData := make(map[string]interface{})
	logMatched := response.Hits.Hits[0]

	err = json.Unmarshal(logMatched.Source, &logData)
	if err != nil {
		return nil, &LogError{
			Err:  errors.New("Error occurred while unmarshalling log hit"),
			Code: http.StatusInternalServerError,
		}
	}

	// Add the ID
	logData["id"] = logMatched.Id

	logData = ParseHeaderString(logData, "headers_string", "header")

	// Marshal and return
	rawLog, err := json.Marshal(logData)
	if err != nil {
		return nil, &LogError{
			Err:  errors.New("error occurred while marshalling log body"),
			Code: http.StatusInternalServerError,
		}
	}

	// If the user passed a flag to parse the diffs, we need to
	if parseDiffs {
		rawLog, err = parseStageDiffs(rawLog)
		if err != nil {
			return nil, &LogError{
				Err:  errors.New(fmt.Sprint("error while parsing stage diffs: ", err)),
				Code: http.StatusInternalServerError,
			}
		}
	}

	return rawLog, nil
}

// parseStageDiffs parses the context diffs and returns the contexts
// for each stage.
func parseStageDiffs(logPassed []byte) ([]byte, error) {
	// Parse the log to a pipelineLog object
	var logRecord record

	err := json.Unmarshal(logPassed, &logRecord)
	if err != nil {
		errMsg := fmt.Sprint("error occurred while parsing log to PipelineLog, ", err)
		log.Warn(logTag, ": ", errMsg)
		return logPassed, errors.New(errMsg)
	}

	// Parse the requestChanges
	request := logRecord.Request
	bodyText1 := request.Body
	headerText1, err := json.Marshal(request.Headers)
	if err != nil {
		errMsg := fmt.Sprintf("error while marshalling request headers, %s", err)
		return logPassed, errors.New(errMsg)
	}
	URIText1 := request.URI
	MethodText1 := request.Method

	// Set parse error for request and response changes to
	// nil by default.
	var requestParseError, responseParseError *string

	for changeIndex, change := range logRecord.RequestChanges {
		if change.Body != "" {
			bodyText2, err := util.ApplyDelta(bodyText1, change.Body)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying body delta for stage number %d, %s ", changeIndex+1, err)
				log.Warnln(logTag, errMsg)
				requestParseError = &errMsg
				break
			}

			logRecord.RequestChanges[changeIndex].Body = bodyText2
			bodyText1 = bodyText2
		}

		if change.Headers != "" {
			headerText2, err := util.ApplyDelta(string(headerText1), change.Headers)
			if err != nil {
				errMsg := fmt.Sprint("error while applying header delta for stage number: ", err)
				log.Warnln(logTag, errMsg)
				requestParseError = &errMsg
				break
			}

			logRecord.RequestChanges[changeIndex].Headers = headerText2
			headerText1 = []byte(headerText2)
		}

		if change.URI != "" {
			URIText2, err := util.ApplyDelta(URIText1, change.URI)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying delta for URI in stage %d, %s", changeIndex+1, err)
				log.Warnln(logTag, errMsg)
				requestParseError = &errMsg
				break
			}

			logRecord.RequestChanges[changeIndex].URI = URIText2
			URIText1 = URIText2
		}

		if change.Method != "" {
			MethodText2, err := util.ApplyDelta(MethodText1, change.Method)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying delta for URI in stage %d, %s", changeIndex+1, err)
				log.Warnln(logTag, errMsg)
				requestParseError = &errMsg
				break
			}

			logRecord.RequestChanges[changeIndex].Method = MethodText2
			MethodText1 = MethodText2
		}
	}

	// If there is an error with the request, set the changes to an
	// empty array
	if requestParseError != nil {
		logRecord.RequestChanges = make([]difference.Difference, 0)
	}

	// Parse the response changes
	response := logRecord.Response
	responseBodyText1 := response.Body
	responseHeaderText1, err := json.Marshal(response.Headers)
	if err != nil {
		errMsg := fmt.Sprintf("error while marshalling response headers, %s", err)
		return logPassed, errors.New(errMsg)
	}

	// Iterate response changes in reverse order since we get the final response
	// in the root log.
	//
	// We also need to keep updating the body and header
	for changeIndex := len(logRecord.ResponseChanges) - 1; changeIndex >= 0; changeIndex-- {
		change := logRecord.ResponseChanges[changeIndex]
		if change.Body != "" {
			bodyText2, err := util.ApplyDelta(responseBodyText1, change.Body)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying body delta to response for stage number %d,  %s", changeIndex+1, err)
				log.Warnln(logTag, errMsg)
				responseParseError = &errMsg
			}

			logRecord.ResponseChanges[changeIndex].Body = bodyText2
			responseBodyText1 = bodyText2
		}

		if change.Headers != "" {
			headerText2, err := util.ApplyDelta(string(responseHeaderText1), change.Headers)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying response header delta for stage number %d, %s ", changeIndex+1, err)
				log.Warnln(logTag, errMsg)
				responseParseError = &errMsg
			}

			logRecord.ResponseChanges[changeIndex].Headers = headerText2
			responseHeaderText1 = headerText1
		}
	}

	// If response parsing has error, set the changes as empty array.
	if responseParseError != nil {
		logRecord.ResponseChanges = make([]difference.Difference, 0)
	}

	updatedLogInBytes, err := json.Marshal(logRecord)
	if err != nil {
		errMsg := fmt.Sprint("error while marshalling updated log, ", err)
		return logPassed, errors.New(errMsg)
	}

	// Parse the context to interface instead of keeping them as string
	logMap := make(map[string]interface{})
	unmarshallLogErr := json.Unmarshal(updatedLogInBytes, &logMap)
	if unmarshallLogErr != nil {
		errMsg := fmt.Sprint("error while unmarshalling log to parse context to interface, ", unmarshallLogErr)
		return updatedLogInBytes, errors.New(errMsg)
	}

	// Set the request and response parse errors.
	logMap["requestParseError"] = requestParseError
	logMap["responseParseError"] = responseParseError

	// Parse the strings to JSON
	logMap["requestChanges"], err = parseStringToMap(logMap["requestChanges"])
	if err != nil {
		errMsg := fmt.Sprint("error while parsing request changes, ", err)
		return nil, errors.New(errMsg)
	}

	logMap["responseChanges"], err = parseStringToMap(logMap["responseChanges"])
	if err != nil {
		errMsg := fmt.Sprint("error while parsing response changes, ", err)
		return nil, errors.New(errMsg)
	}

	finalLogInBytes, err := json.Marshal(logMap)
	if err != nil {
		errMsg := fmt.Sprint("error while marshaling the final log, ", err)
		log.Warnln(logTag, ": ", errMsg)
		return updatedLogInBytes, errors.New(errMsg)
	}

	return finalLogInBytes, nil
}

// parseStringToMap Parses JSON for the passed map
func parseStringToMap(changes interface{}) (interface{}, error) {
	requestChanges, ok := changes.([]interface{})
	if !ok {
		errMsg := fmt.Sprint("error while converting request changes to interface array")
		return requestChanges, errors.New(errMsg)
	}

	for changeIndex, change := range requestChanges {
		changeAsMap, ok := change.(map[string]interface{})
		if !ok {
			errMsg := fmt.Sprint("error while converting stage to map from interface")
			return requestChanges, errors.New(errMsg)
		}

		// Convert the string to map
		bodyAsString := changeAsMap["body"].(string)

		// NOTE: Cannot do the opposite check and make the iteration
		// skipped using continue because we will be parsing
		// headers as well.
		if bodyAsString != "" {
			bodyAsMap := make(map[string]interface{})
			err := json.Unmarshal([]byte(bodyAsString), &bodyAsMap)
			if err != nil {
				// It's possible that the body is nd-json in which case, we will
				// not raise an error and return the body as string.
				errMsg := fmt.Sprintf("error while parsing body to map from string for stage %d with err: %s", changeIndex+1, err)
				log.Warnln(logTag, ": ", errMsg, " Returning as string.")
			} else {
				changeAsMap["body"] = bodyAsMap
			}
		}

		// Convert the headers to map
		headersAsString := changeAsMap["headers"].(string)

		// NOTE: Since there are no following actions, we can skip the iteration
		// if headers is empty,
		if headersAsString == "" {
			continue
		}

		headersAsMap := make(map[string]interface{})
		err := json.Unmarshal([]byte(headersAsString), &headersAsMap)
		if err != nil {
			// If headers failed, throw error since this should always be a map.
			errMsg := fmt.Sprintf("error while parsing headers for stage %d with err: %s", changeIndex+1, err)
			return nil, errors.New(errMsg)
		}

		changeAsMap["headers"] = headersAsMap

		requestChanges[changeIndex] = changeAsMap
	}

	return requestChanges, nil
}

type LogError struct {
	Err  error
	Code int
}

// To handle the breaking change to return `headers_string` as a map named `header`
func ParseHeaderString(logData map[string]interface{}, headersStringKey, headersMapKey string) map[string]interface{} {
	IsUsingStringHeaders, ok := logData["is_using_stringified_headers"].(bool)
	if ok && IsUsingStringHeaders {
		if logData["request"] != nil {
			requestAsMap, ok := logData["request"].(map[string]interface{})
			if ok {
				headersAsString, ok := requestAsMap[headersStringKey].(string)
				if ok {
					var headersMap map[string][]string
					err := json.Unmarshal([]byte(headersAsString), &headersMap)
					if err != nil {
						log.Errorln(logTag, ":", err)
					} else {
						// write header for header string
						logData["request"].(map[string]interface{})[headersMapKey] = headersMap
						delete(logData["request"].(map[string]interface{}), headersStringKey)
					}
				}
			}
		}
		if logData["response"] != nil {
			requestAsMap, ok := logData["response"].(map[string]interface{})
			if ok {
				headersAsString, ok := requestAsMap[headersStringKey].(string)
				if ok {
					var headersMap map[string][]string
					err := json.Unmarshal([]byte(headersAsString), &headersMap)
					if err != nil {
						log.Errorln(logTag, ":", err)
					} else {
						// write header for header string
						logData["response"].(map[string]interface{})[headersMapKey] = headersMap
						delete(logData["response"].(map[string]interface{}), headersStringKey)
					}
				}
			}
		}
	}
	return logData
}
