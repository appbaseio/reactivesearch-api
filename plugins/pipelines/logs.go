package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/middleware/classify"
	"github.com/appbaseio-confidential/reactivesearch/model/acl"
	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/plugins/logs"
	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// Structure to store the pipeline logs
type PipelineLog struct {
	Route        *string            `json:"route"`
	PipelineID   *string            `json:"pipeline_id"`
	Category     *category.Category `json:"category"`
	ACL          *acl.ACL           `json:"acl"`
	Took         *int               `json:"took"`
	Response     *Response          `json:"response"`
	Request      *Request           `json:"request"`
	Context      *string            `json:"context"`
	StageChanges *[]StageChange     `json:"stageChanges"`
	Timestamp    time.Time          `json:"timestamp"`
	DiffLogs     bool               `json:"diffLogs"`
}

// Response stores the response returned by the pipeline
type Response struct {
	Body    string              `json:"body"`
	Headers map[string][]string `json:"headers"`
	Code    int                 `json:"code"`
}

// Request stores the request passed to the pipeline
type Request struct {
	Method  string              `json:"method"`
	Body    string              `json:"body"`
	Headers map[string][]string `json:"headers"`
	URI     string              `json:"uri"`
}

// StageChange stores the changes happened in the stage
type StageChange struct {
	ID      string  `json:"id"`
	Error   *string `json:"error"`
	Context *string `json:"context"`
	Took    *int    `json:"took"`
}

// logsFilter will store the logs filter data as passed
// by the user.
type logsFilter struct {
	Offset           int
	StartDate        string
	EndDate          string
	OrderByLatency   string
	Size             int
	Filter           string
	OrderByTimestamp string
	PipelineID       string
}

// getPipelineLogs will get the pipeline logs from the database
// based on the passed filters
func (es *logsElasticsearch) getPipelineLogs(ctx context.Context, logsFilter logsFilter) ([]byte, error) {
	// Add start and end date to the filter
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
	} else if logsFilter.PipelineID == "" {
		query.Filter(es7.NewMatchAllQuery())
	}

	if logsFilter.PipelineID != "" {
		pipelineIDFilter := es7.NewTermsQuery("pipeline_id.keyword", logsFilter.PipelineID)
		query.Filter(pipelineIDFilter)
	}

	searchQuery := util.GetClient7().Search(es.indexName).
		Query(query).
		From(logsFilter.Offset).
		Size(logsFilter.Size)

	// If orderByLatency is set then set it in the query
	if logsFilter.OrderByLatency != "" {
		ascending := false
		if logsFilter.OrderByLatency == "asc" {
			ascending = true
		}
		// sort by latency
		searchQuery.SortWithInfo(es7.SortInfo{Field: "took", UnmappedType: "int", Ascending: ascending})
	}

	if logsFilter.OrderByTimestamp != "" {
		ascending := false
		if logsFilter.OrderByTimestamp == "asc" {
			ascending = true
		}

		searchQuery.SortWithInfo(es7.SortInfo{Field: "timestamp", UnmappedType: "date", Ascending: ascending})
	}

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

// getPipelineLogById will get the pipeline log by matching it
// against the passed ID.
//
// This method will raise a 404 if no log is found for the passed
// ID.
func (es *logsElasticsearch) getPipelineLogById(ctx context.Context, logId string, parseDiffs bool) ([]byte, *Error) {
	// Create the query
	query := es7.NewTermQuery("_id", logId)
	response, err := util.GetClient7().Search(es.indexName).Query(query).Size(1).Do(ctx)

	if err != nil {
		errCode := http.StatusInternalServerError

		log.Errorln(logTag, ": error while getting log by ID")
		return nil, &Error{
			Err:  err,
			Code: errCode,
		}
	}

	if len(response.Hits.Hits) == 0 {
		return nil, &Error{
			Err:  errors.New(fmt.Sprintf("Log not found with ID: %s", logId)),
			Code: http.StatusNotFound,
		}
	}

	log := make(map[string]interface{})
	logMatched := response.Hits.Hits[0]

	err = json.Unmarshal(logMatched.Source, &log)
	if err != nil {
		return nil, &Error{
			Err:  errors.New("Error occurred while unmarshalling log hit"),
			Code: http.StatusInternalServerError,
		}
	}

	// Add the ID
	log["id"] = logMatched.Id

	// Marshal and return
	rawLog, err := json.Marshal(log)
	if err != nil {
		return nil, &Error{
			Err:  errors.New("error occurred while marshalling log body"),
			Code: http.StatusInternalServerError,
		}
	}

	// If the user passed a flag to parse the diffs, we need to
	if parseDiffs {
		rawLog, err = parseContextDiffs(rawLog)
		if err != nil {
			return rawLog, &Error{
				Err:  err,
				Code: http.StatusInternalServerError,
			}
		}
	}

	return rawLog, nil
}

// rolloverIndexJob will handle the rolling over of the pipeline logs index
// in ElasticSearch.
func (es *logsElasticsearch) rolloverIndexJob(alias string) {
	ctx := context.Background()
	rolloverConditions := make(map[string]interface{})
	rolloverConfiguration := fmt.Sprintf(rolloverConfig, "7d", 10000, "1gb")
	if util.IsProductionPlan() {
		rolloverConfiguration = fmt.Sprintf(rolloverConfig, "30d", 1000000, "10gb")
	}
	json.Unmarshal([]byte(rolloverConfiguration), &rolloverConditions)
	settingsString := fmt.Sprintf(`{%s "index.number_of_shards": 2, "index.number_of_replicas": %d}`, util.HiddenIndexSettings(), util.GetReplicas())
	settings := make(map[string]interface{})
	json.Unmarshal([]byte(settingsString), &settings)

	mappingString := pipelineLogsMapping

	mappings := make(map[string]interface{})
	json.Unmarshal([]byte(mappingString), &mappings)
	rolloverService, err := es7.NewIndicesRolloverService(util.GetClient7()).
		Alias(alias).
		Conditions(rolloverConditions).
		Settings(settings).
		Mappings(mappings).
		Do(ctx)
	if err != nil {
		log.Println(logTag, "error while creating a rollover service", alias, err)
	}
	log.Println(logTag, ": rollover res oldIndex", rolloverService.OldIndex)
	log.Println(logTag, ": rollover res newIndex", rolloverService.NewIndex)
	log.Println(logTag, ": rollover res isRolledover", rolloverService.RolledOver)

	if rolloverService.RolledOver {
		classify.SetIndexAlias(rolloverService.NewIndex, alias)
		classify.SetAliasIndex(alias, rolloverService.NewIndex)
	}

	// We cannot rely on rollover service response here,
	// Because it returns rollover as false when we restart ReactiveSearch.
	// To preserve the last 2 index and delete others:
	// -> cat all the indices with .logs-*
	// -> if count is > 2
	//   -> sort them based on -[Number]
	//   -> preserve last 2 and delete all
	// -> else do not delete any index

	// cat all the indices starting with `${alias}-Number` pattern
	indices, err := util.GetClient7().CatIndices().Index(alias + "-*").
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": rollover cronjob error getting indices", err)
	}

	if len(indices) > 2 {
		rolloverIndices := []string{}
		r, _ := regexp.Compile(fmt.Sprintf("%s-[0-9]+", alias))
		for _, catResRow := range indices {
			if r.MatchString(catResRow.Index) {
				rolloverIndices = append(rolloverIndices, catResRow.Index)
			}
		}

		sort.Strings(rolloverIndices)

		// ignore last 2 indices
		rolloverIndices = rolloverIndices[:len(rolloverIndices)-2]

		log.Println(logTag, ": rollover cronjob, indices to delete", rolloverIndices)
		_, err = util.GetClient7().DeleteIndex(strings.Join(rolloverIndices, ",")).Do(ctx)
		if err != nil {
			log.Errorln(logTag, ": rollover cronjob, error while deleting indices", err)
		}
	}
}

// parseSortBy validates the sortBy value passed by
// the user and updates the logsFilter accordingly
func parseSortBy(sortByPassed string, logsFilter *logsFilter) error {
	// Map the values accordingly
	switch sortByPassed {
	case "latency_asc":
		logsFilter.OrderByLatency = "asc"
	case "latency_desc":
		logsFilter.OrderByLatency = "desc"
	case "timestamp_asc":
		logsFilter.OrderByTimestamp = "asc"
	case "timestamp_desc":
		logsFilter.OrderByTimestamp = "desc"
	default:
		return errors.New(fmt.Sprintf("invalid sort_by value: %s", sortByPassed))
	}

	return nil
}

// GetStageChange will get the stageChange for the passed ID.
//
// It will first check if the stage is already present, if so
// it will return that stage.
// If it is not present, then a new one will be created, appended
// to the array and then returned.
func GetStageChange(stageID string, stageChanges *[]*StageChange) *StageChange {
	for _, stage := range *stageChanges {
		if stage.ID == stageID {
			return stage
		}
	}

	// Else create a new one, append and return
	stageChange := StageChange{
		ID:    stageID,
		Error: nil,
	}
	*stageChanges = append(*stageChanges, &stageChange)

	return &stageChange
}

// initLogsRecorder will init the logs recorder
func (route ESPipelineRoutes) initLogsRecorder(h http.HandlerFunc) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {

		// If recording logs is disabled, return
		if route.RecordLogs != nil && !*route.RecordLogs {
			return
		}

		// Create a new pipeline log and store it in the
		// context with the basic pipeline details
		pipelineLog := PipelineLog{
			Route:        route.Path,
			Category:     route.Classify.Category,
			ACL:          route.Classify.ACL,
			Request:      new(Request),
			PipelineID:   new(string),
			Took:         new(int),
			StageChanges: new([]StageChange),
		}
		logCtx := NewContext(r.Context(), &pipelineLog)
		r = r.WithContext(logCtx)

		// Inject the waitgroup to use for waiting for context diffs
		wgArr := []*sync.WaitGroup{new(sync.WaitGroup), new(sync.WaitGroup), new(sync.WaitGroup)}
		wgCtx := WgNewContext(r.Context(), &wgArr)
		r = r.WithContext(wgCtx)

		respRecorder := httptest.NewRecorder()
		h(respRecorder, r)
		// Copy the response to writer
		for k, v := range respRecorder.Header() {
			rw.Header()[k] = v
		}
		rw.WriteHeader(respRecorder.Code)
		rw.Write(respRecorder.Body.Bytes())

		// Record the document
		go captureResponse(respRecorder, r)
	}
}

// captureResponse will capture the response, extract the logged
// pipeline from the context and write it to the logs
func captureResponse(rw *httptest.ResponseRecorder, r *http.Request) {
	// Write the log using lumberjack
	logBody, err := FromContext(r.Context())
	if err != nil {
		log.Warnln(logTag, "couldn't read logbody from ctx with err: ", err)
		return
	}

	// Extract the wg and wait for the changes
	logWgArr, wgErr := WgFromContext(r.Context())
	if wgErr != nil {
		log.Warnln(logTag, "couldn't read log update waitgroup from ctx with err: ", err)
		return
	}

	log.Debug(logTag, " waitgroup address: ", (*logWgArr)[0])
	log.Debug(logTag, ": waiting for log changes to complete")
	startTime := time.Now()
	(*logWgArr)[0].Wait()
	log.Debug("Time waited for log update: ", int(time.Since(startTime).Milliseconds()))

	// Assign timestamp to the logbody
	logBody.Timestamp = time.Now()

	// Extract response
	logBody.Response = new(Response)

	response := rw.Result()
	logBody.Response.Code = response.StatusCode
	logBody.Response.Headers = response.Header

	// Marshal the response body
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln(logTag, "can't read response body: ", err)
		return
	}

	logBody.Response.Body = string(responseBody)

	// Marshal the logbody
	marshalledLog, err := json.Marshal(logBody)
	if err != nil {
		log.Errorln(logTag, "error while marshalling log body, ", err)
		return
	}

	lumberjack := Instance().lumberjack

	_, err = lumberjack.Write(marshalledLog)
	if err != nil {
		log.Errorln(logTag, "error encountered while writing logs :", err)
		return
	}
	// Add new line character so fluentbit can track the read state (similar to how we do it with logs)
	lumberjack.Write([]byte("\n"))
	log.Infoln(logTag, " pipeline request logged sucessfully!")
}

// parseContextDiffs parses the context diffs and returns the contexts
// for each stage.
func parseContextDiffs(logPassed []byte) ([]byte, error) {
	// Parse the log to a pipelineLog object
	var pipelineLog PipelineLog

	err := json.Unmarshal(logPassed, &pipelineLog)
	if err != nil {
		errMsg := fmt.Sprint("error occurred while parsing log to PipelineLog, ", err)
		log.Warn(logTag, ": ", errMsg)
		return logPassed, errors.New(errMsg)
	}

	text1 := *pipelineLog.Context

	// Set the parseError as nil by default
	parseError := new(string)

	// Check if the stageChanges are nil
	if pipelineLog.StageChanges == nil {
		emptyStageChanges := make([]StageChange, 0)
		pipelineLog.StageChanges = &emptyStageChanges
	}

	isDiffingDisabled := logs.Instance().IsDiffingDisabled()

	// If Diffing is disabled then we don't need to apply the
	// delta otherwise iterate through the changes and apply the
	// delta on each stage.
	if !isDiffingDisabled {
		for stageIndex, stage := range *pipelineLog.StageChanges {
			if stage.Context == nil {
				continue
			}

			// Calculate updated context and update it.
			contextCalculated, err := util.ApplyDelta(text1, *stage.Context)
			if err != nil {
				errMsg := fmt.Sprintf("error occurred while calculating context for stage number %d, %s", stageIndex, err)
				log.Warn(logTag, ": ", errMsg)

				// Don't return with error
				// Just return the stageChanges as empty and return the error
				// in the parseError field.
				parseError = &errMsg
				emptyStageChanges := make([]StageChange, 0)
				*pipelineLog.StageChanges = emptyStageChanges
				break
			}

			// Update text1 if error is not null
			if stage.Error == nil {
				text1 = contextCalculated
			}

			// Update the stage as well
			*(*pipelineLog.StageChanges)[stageIndex].Context = contextCalculated
		}
	}

	// Finally marshal the pipeline log.
	updatedLogInBytes, err := json.Marshal(pipelineLog)
	if err != nil {
		errMsg := fmt.Sprint("error while marshalling updated pipeline log, ", err)
		log.Warnln(logTag, ": ", errMsg)
		return logPassed, err
	}

	// Parse the context to interface instead of keeping them as string
	logMap := make(map[string]interface{})
	unmarshallLogErr := json.Unmarshal(updatedLogInBytes, &logMap)
	if unmarshallLogErr != nil {
		errMsg := fmt.Sprint("error while unmarshalling log to parse context to interface, ", unmarshallLogErr)
		return updatedLogInBytes, errors.New(errMsg)
	}

	// Set the parseError field
	logMap["parseError"] = parseError

	stageChanges, ok := logMap["stageChanges"].([]interface{})
	if !ok {
		errMsg := "error while converting stageChanges to a map"
		log.Warnln(logTag, ": ", errMsg)
		return nil, errors.New(errMsg)
	}

	for stageIndex, stage := range stageChanges {
		stageChange, ok := stage.(map[string]interface{})
		if !ok {
			errMsg := fmt.Sprintf("error while converting stagechange to map for stage number %d", stageIndex)
			log.Warnln(logTag, ": ", errMsg)
			return nil, errors.New(errMsg)
		}

		if stageChange["context"] == nil {
			continue
		}

		contextAsString := stageChange["context"].(string)

		contextAsMap := make(map[string]interface{})
		err := json.Unmarshal([]byte(contextAsString), &contextAsMap)
		if err != nil {
			errMsg := fmt.Sprint("error while unmarshalling context, ", err)
			log.Warnln(logTag, ": ", errMsg)
			return nil, errors.New(errMsg)
		}

		stageChange["context"] = contextAsMap
		stageChanges[stageIndex] = stageChange
	}

	// Update the stageChanges
	logMap["stageChanges"] = stageChanges

	// Marshal again
	finalLogInBytes, err := json.Marshal(logMap)
	if err != nil {
		errMsg := fmt.Sprint("error while marshalling after parsing context for stage changes, ", err)
		log.Warnln(logTag, ": ", errMsg)
		return nil, errors.New(errMsg)
	}

	return finalLogInBytes, nil
}
