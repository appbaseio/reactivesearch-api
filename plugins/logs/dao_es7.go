package logs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"

	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) getRawLogsES7(ctx context.Context, logsFilter logsFilter) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(logsFilter.StartDate).
		To(logsFilter.EndDate)

	query := es7.NewBoolQuery().Filter(duration)
	// apply category filter
	if logsFilter.Filter == "search" {
		filters := es7.NewTermsQuery("category.keyword", []interface{}{"search", category.ReactiveSearch.String(), "suggestion"}...)
		query.Filter(filters)
	} else if logsFilter.Filter == "suggestion" {
		filters := es7.NewTermsQuery("category.keyword", []interface{}{"suggestion"}...)
		query.Filter(filters)
	} else if logsFilter.Filter == "index" {
		filters := []es7.Query{
			es7.NewTermsQuery("request.method.keyword", []interface{}{"POST", "PUT"}...),
			es7.NewTermsQuery("category.keyword", []interface{}{"docs"}...),
			es7.NewRangeQuery("response.code").Gte(200).Lte(299),
		}
		query.Filter(filters...)
	} else if logsFilter.Filter == "delete" {
		filters := es7.NewMatchQuery("request.method.keyword", "DELETE")
		query.Filter(filters)
	} else if logsFilter.Filter == "success" {
		filters := es7.NewRangeQuery("response.code").Gte(200).Lte(299)
		query.Filter(filters)
	} else if logsFilter.Filter == "error" {
		filters := es7.NewRangeQuery("response.code").Gte(400)
		query.Filter(filters)
	} else {
		query.Filter(es7.NewMatchAllQuery())
	}

	// apply index filtering logic
	util.GetIndexFilterQueryEs7(query, logsFilter.Indices...)

	// only apply latency filter when start or end range is available
	if logsFilter.StartLatency != nil || logsFilter.EndLatency != nil {
		latencyRangeQuery := es7.NewRangeQuery("response.took")
		if logsFilter.StartLatency != nil {
			latencyRangeQuery.Gte(*logsFilter.StartLatency)
		}
		if logsFilter.EndLatency != nil {
			latencyRangeQuery.Lte(*logsFilter.EndLatency)
		}
		query.Filter(latencyRangeQuery)
	}

	searchQuery := util.GetClient7().Search(es.indexName).
		Query(query).
		From(logsFilter.Offset).
		Size(logsFilter.Size)
	if logsFilter.OrderByLatency != "" {
		ascending := false
		if logsFilter.OrderByLatency == "asc" {
			ascending = true
		}
		// sort by latency
		searchQuery.SortWithInfo(es7.SortInfo{Field: "response.took", UnmappedType: "int", Ascending: ascending})
	}
	searchQuery.SortWithInfo(es7.SortInfo{Field: "timestamp", UnmappedType: "date", Ascending: false})
	response, err := searchQuery.Do(ctx)
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
	response, err := util.GetClient7().Get().Index(es.indexName).Id(ID).Do(ctx)

	if err != nil {
		errCode := http.StatusInternalServerError

		// Check if 404 using the err message
		// and change err code depending on that.
		isNotFound, _ := regexp.MatchString(`.*404.*`, err.Error())
		if isNotFound {
			errCode = http.StatusNotFound
		}

		log.Errorln(logTag, ": error while getting log by ID")
		return nil, &LogError{
			Err:  err,
			Code: errCode,
		}
	}

	log := make(map[string]interface{})
	err = json.Unmarshal(response.Source, &log)
	if err != nil {
		return nil, &LogError{
			Err:  errors.New("Error occurred while unmarshalling log hit"),
			Code: http.StatusInternalServerError,
		}
	}

	// Add the ID
	log["id"] = response.Id

	// Marshal and return
	rawLog, err := json.Marshal(log)
	if err != nil {
		return nil, &LogError{
			Err:  errors.New("error occurred while marshalling log body"),
			Code: http.StatusInternalServerError,
		}
	}

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

	for changeIndex, change := range logRecord.RequestChanges {
		if change.Body != "" {
			bodyText2, err := util.ApplyDelta(bodyText1, change.Body)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying body delta for stage number %d, %s ", changeIndex, err)
				return logPassed, errors.New(errMsg)
			}

			logRecord.RequestChanges[changeIndex].Body = bodyText2
			bodyText1 = bodyText2
		}

		if change.Headers != "" {
			headerText2, err := util.ApplyDelta(string(headerText1), change.Headers)
			if err != nil {
				errMsg := fmt.Sprint("error while applying header delta for stage number: ", err)
				return logPassed, errors.New(errMsg)
			}

			logRecord.RequestChanges[changeIndex].Headers = headerText2
			headerText1 = []byte(headerText2)
		}

		if change.URI != "" {
			URIText2, err := util.ApplyDelta(URIText1, change.URI)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying delta for URI in stage %d, %s", changeIndex, err)
				return logPassed, errors.New(errMsg)
			}

			logRecord.RequestChanges[changeIndex].URI = URIText2
			URIText1 = URIText2
		}

		if change.Method != "" {
			MethodText2, err := util.ApplyDelta(MethodText1, change.Method)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying delta for URI in stage %d, %s", changeIndex, err)
				return logPassed, errors.New(errMsg)
			}

			logRecord.RequestChanges[changeIndex].Method = MethodText2
			MethodText1 = MethodText2
		}
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
				errMsg := fmt.Sprintf("error while applying body delta to response for stage number %d,  %s", changeIndex, err)
				return logPassed, errors.New(errMsg)
			}

			logRecord.ResponseChanges[changeIndex].Body = bodyText2
			responseBodyText1 = bodyText2
		}

		if change.Headers != "" {
			headerText2, err := util.ApplyDelta(string(responseHeaderText1), change.Headers)
			if err != nil {
				errMsg := fmt.Sprintf("error while applying response header delta for stage number %d, %s ", changeIndex, err)
				return logPassed, errors.New(errMsg)
			}

			logRecord.ResponseChanges[changeIndex].Headers = headerText2
			responseHeaderText1 = headerText1
		}
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

		if bodyAsString == "" {
			continue
		}

		bodyAsMap := make(map[string]interface{})
		err := json.Unmarshal([]byte(bodyAsString), &bodyAsMap)
		if err != nil {
			// It's possible that the body is nd-json in which case, we will
			// not raise an error and return the body as string.
			errMsg := fmt.Sprint("error while parsing body to map from string, ", err)
			log.Warnln(logTag, ": ", errMsg, " Returning as string.")
		} else {
			changeAsMap["body"] = bodyAsMap
		}

		requestChanges[changeIndex] = changeAsMap
	}

	return requestChanges, nil
}

type LogError struct {
	Err  error
	Code int
}
