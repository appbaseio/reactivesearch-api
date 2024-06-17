package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/analytics"
	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// Store the pipeline invocation details when a
// pipeline invocation is successfull.
type PipelineInvoke struct {
	ID        *string                        `json:"pipeline_id,omitempty"`
	Version   *int                           `json:"version,omitempty"`
	Stages    map[string]PipelineInvokeStage `json:"stages,omitempty"`
	TimeStamp int64                          `json:"timestamp,omitempty"`
	Took      int                            `json:"took,omitempty"`
	Error     *bool                          `json:"error,omitempty"`
}

// Store the stage details for the pipeline invocation
type PipelineInvokeStage struct {
	Error    *bool `json:"error,omitempty"`
	Took     *int  `json:"took,omitempty"`
	Executed *bool `json:"executed,omitempty"`
}

// PipelineInvokeMap stores the map of pipeline stage ID's
// to pipeline invoke stage and makes use of a mutex to lock
// and unlock while writing
type PipelineInvokeMap struct {
	lock  sync.Mutex
	value map[string]PipelineInvokeStage
}

// AddStage adds a stage to the invoke map
func (p *PipelineInvokeMap) AddStage(stageId string, isStageError *bool, stageTook *int, isStageExecuted *bool) {
	p.lock.Lock()
	p.value[stageId] = PipelineInvokeStage{
		Error:    isStageError,
		Took:     stageTook,
		Executed: isStageExecuted,
	}
	p.lock.Unlock()
}

func (p *PipelineInvokeMap) GetStagesMap() *map[string]PipelineInvokeStage {
	return &p.value
}

// createInvocationRecord will create an invocation record with
// the passed content.
//
// The timestamp field in the record will be updated by this function
// so it can be passed as nil.
func (es *invocationElasticsearch) createInvocationRecord(ctx context.Context, record PipelineInvoke) error {
	// Update the timestamp
	// Convert the timestamp to milliseconds since we're using go 1.16
	// and it doesn't have the Milli() method
	timeNow := time.Now().UnixNano() / int64(time.Millisecond)
	record.TimeStamp = timeNow

	// Marshal the record and write it through lumberjack
	marshalledInvocation, marshalErr := json.Marshal(record)
	if marshalErr != nil {
		log.Warnln(logTag, ": error while marshalling invocation record to write: ", marshalErr.Error())
		return marshalErr
	}

	_, err := Instance().invocationLumberjack.Write(marshalledInvocation)

	if err != nil {
		log.Errorln(logTag, "error encountered while writing invocation record: ", err)
		return nil
	}
	// Add new line character so fluent-bit can track the read state (similar to how we do it with logs)
	Instance().invocationLumberjack.Write([]byte("\n"))
	log.Info(logTag, ": invocation request written successfully!")

	return nil
}

// createPipelineInvokeRecord is a wrapper around the createInvocationRecord
// function that creates a new record based on the passed pipeline ID and stages.
func (es *invocationElasticsearch) createPipelineInvokeRecord(pipelineIDPassed string, version int, stages *map[string]PipelineInvokeStage, took int) error {
	// Create a new record with the passed details.
	invokeRecord := PipelineInvoke{
		ID:      &pipelineIDPassed,
		Stages:  *stages,
		Took:    took,
		Version: &version,
	}

	// Determine whether there was an error or not.
	//
	// If at least one stage in the pipeline stages has the
	// error set to True, we will consider the pipeline as errored
	// out.
	isError := false
	for _, stageRun := range *stages {
		if *stageRun.Error {
			isError = true
			break
		}
	}

	invokeRecord.Error = &isError

	// Create the record
	go func() {
		err := es.createInvocationRecord(context.Background(), invokeRecord)
		if err != nil {
			log.Warnln(logTag, ": error received while storing invocation record: ", err.Error())
		}
	}()

	return nil
}

// queryPipelinesUsage returns the usage of the pipeline by grouping
// them on the ID and counting their invocations in the given timestamp.
func (es *invocationElasticsearch) queryPipelinesUsage(ctx context.Context, from, to string, size int, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	analytics.ApplyCustomEventsEs7(query, filters)

	aggr := es7.NewTermsAggregation().
		Field("pipeline_id").
		Size(size).
		OrderByCountDesc()

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("pipelines_aggr", aggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("pipelines_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'pipelines_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	storedQueries := make(map[string]interface{})
	if buckets == nil {
		storedQueries["pipelines"] = []interface{}{}
	} else {
		storedQueries["pipelines"] = buckets
	}
	return json.Marshal(storedQueries)
}

// queryPipelineStageUsage returns the count aggregation of the stages
// array inside the pipeline doc for the passed pipeline ID.
func (es *invocationElasticsearch) queryPipelineStageUsage(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	analytics.ApplyCustomEventsEs7(query, filters)

	// Add filter for the pipeline ID
	query = query.Filter(es7.NewTermQuery("pipeline_id", pipelineID))

	// Get pipeline from cache
	pipelineDoc, _ := GetPipelineAndLocFromCache(pipelineID)
	if pipelineDoc == nil {
		return nil, errors.New("couldn't fetch pipeline from cache to extract stage ID's")
	}

	// Extract the stage ID's
	stageIds := make([]string, len(*pipelineDoc.Stages))
	for stageIndex, stage := range *pipelineDoc.Stages {
		stageIds[stageIndex] = *stage.ID
	}

	// Create nested aggregation for stages
	stagesAggr := es7.NewTermsAggregation().
		Field("pipeline_id").
		Size(size).
		OrderByCountDesc()

	// Add the sub aggregations
	for _, stageId := range stageIds {
		stagesAggr = stagesAggr.
			SubAggregation(stageId, es7.NewValueCountAggregation().Field(fmt.Sprintf("stages.%s.error", stageId)))
	}

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("pipelines", stagesAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	// Extract the pipelines buckets
	aggrResult, found := result.Aggregations.Terms("pipelines")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'pipelines'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		// Extract every stage for the pipeline
		for _, stageId := range stageIds {
			value, found := bucket.ValueCount(stageId)
			if !found {
				return nil, errors.New(fmt.Sprint("error while fetching value for stage: ", stageId))
			}

			newBucket := make(map[string]interface{})

			newBucket["key"] = stageId
			newBucket["count"] = value.Value

			buckets = append(buckets, newBucket)
		}
	}

	storedQueries := make(map[string]interface{})
	if buckets == nil {
		storedQueries["stages"] = []interface{}{}
	} else {
		storedQueries["stages"] = buckets
	}

	// Add the pipeline ID in response
	storedQueries["pipeline_id"] = pipelineID

	return json.Marshal(storedQueries)
}

// queryPipelineVersionUsage returns the count aggregation of the versions
// for a particular pipeline
func (es *invocationElasticsearch) queryPipelineVersionUsage(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	analytics.ApplyCustomEventsEs7(query, filters)

	// Add filter for the pipeline ID
	query = query.Filter(es7.NewTermQuery("pipeline_id", pipelineID))

	// Create nested aggregation for stages
	versionAggr := es7.NewTermsAggregation().
		Field("version").
		Size(size).
		OrderByCountDesc()

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("version_aggr", versionAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("version_aggr")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'version_aggr'")
	}

	var buckets []map[string]interface{}
	for _, bucket := range aggrResult.Buckets {
		newBucket := make(map[string]interface{})
		newBucket["key"] = bucket.Key
		newBucket["count"] = bucket.DocCount
		buckets = append(buckets, newBucket)
	}

	storedQueries := make(map[string]interface{})
	if buckets == nil {
		storedQueries["versions"] = []interface{}{}
	} else {
		storedQueries["versions"] = buckets
	}
	return json.Marshal(storedQueries)
}

// queryPipelineVersionTimeTaken will query the pipelines time taken values
// and return them based on versions.
func (es *invocationElasticsearch) queryPipelineVersionTimeTaken(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	analytics.ApplyCustomEventsEs7(query, filters)

	// Add filter for the pipeline ID
	query = query.Filter(es7.NewTermQuery("pipeline_id", pipelineID))

	avgTimeTakenAggr := es7.NewAvgAggregation().Field("took")
	versionSubAggr := es7.NewTermsAggregation().Field("version").SubAggregation("time_taken", avgTimeTakenAggr)
	timeTakenAggr := es7.NewDateHistogramAggregation().Field("timestamp").CalendarInterval("day").SubAggregation("version", versionSubAggr)

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("time_taken_per_month", timeTakenAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.DateHistogram("time_taken_per_month")
	if !found {
		return nil, fmt.Errorf("unable to fetch aggregation value from 'time_taken_per_month'")
	}

	topLevelHistogram := make([]interface{}, 0)

	for _, dayBucket := range aggrResult.Buckets {
		dayMap := map[string]interface{}{
			"key":           dayBucket.Key,
			"key_as_string": dayBucket.KeyAsString,
		}

		versionAggr, found := dayBucket.Aggregations.Terms("version")
		if !found {
			return nil, fmt.Errorf("error while fetching version for day bucket")
		}

		versionArr := make([]interface{}, 0)

		for _, versionBucket := range versionAggr.Buckets {
			versionMap := map[string]interface{}{
				"key": versionBucket.Key,
			}

			timeTakenSubAggr, found := versionBucket.Aggregations.Avg("time_taken")
			if !found {
				return nil, fmt.Errorf("error while fetching time_taken for version bucket")
			}

			versionMap["value"] = timeTakenSubAggr.Value

			// Append the versionMap to version array for the day
			versionArr = append(versionArr, versionMap)
		}

		// Add versionArr to dayMap
		dayMap["version_histogram"] = versionArr

		// Add dayMap to topLevelHistogram
		topLevelHistogram = append(topLevelHistogram, dayMap)
	}

	return json.Marshal(topLevelHistogram)
}

// queryPipelineVersionErrorRate will query the pipeline error rate values
// and return them based on versions
func (es *invocationElasticsearch) queryPipelineVersionErrorRate(ctx context.Context, from, to string, size int, pipelineID string, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	analytics.ApplyCustomEventsEs7(query, filters)

	// Add filter for the pipeline ID
	query = query.Filter(es7.NewTermQuery("pipeline_id", pipelineID))

	trueCountAggr := es7.NewValueCountAggregation().Field("error")
	errorCountAggr := es7.NewFilterAggregation().Filter(es7.NewTermQuery("error", true)).SubAggregation("true_count", trueCountAggr)
	versionAggr := es7.NewTermsAggregation().Field("version").SubAggregation("error_count", errorCountAggr)
	dateAggr := es7.NewDateHistogramAggregation().Field("timestamp").CalendarInterval("day").SubAggregation("version", versionAggr)

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("error_per_month", dateAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	errorPerMonthResult, found := result.Aggregations.DateHistogram("error_per_month")
	if !found {
		return nil, fmt.Errorf("error while getting the error_per_month aggregation from ES")
	}

	topLevelHistogram := make([]interface{}, 0)

	for _, dayBucket := range errorPerMonthResult.Buckets {
		dayMap := map[string]interface{}{
			"key":           dayBucket.Key,
			"key_as_string": dayBucket.KeyAsString,
		}

		versionAggr, found := dayBucket.Aggregations.Terms("version")
		if !found {
			return nil, fmt.Errorf("error while getting version info for day buckets")
		}

		versionArr := make([]interface{}, 0)

		for _, versionBucket := range versionAggr.Buckets {
			versionMap := map[string]interface{}{
				"key": versionBucket.Key,
			}

			// Extract the true error count value to calculate
			// error rate for pipeline.
			errorCountResult, found := versionBucket.Aggregations.Filter("error_count")
			if !found {
				return nil, fmt.Errorf("error while getting error_count sub aggregation")
			}

			trueCountResult, found := errorCountResult.Aggregations.ValueCount("true_count")
			if !found {
				return nil, fmt.Errorf("error while getting the true_count sub aggregation")
			}

			errorRateCalculated := *trueCountResult.Value / float64(versionBucket.DocCount) * 100

			// Handle edge case if errorRateCalculated is NaN
			if math.IsNaN(errorRateCalculated) {
				errorRateCalculated = 0
			}

			versionMap["value"] = errorRateCalculated

			// Append versionMap to versionArr
			versionArr = append(versionArr, versionMap)
		}

		dayMap["version_histogram"] = versionArr

		// Add dayMap to topLevelHistogram
		topLevelHistogram = append(topLevelHistogram, dayMap)
	}

	// Finally return the marshalled body
	return json.Marshal(topLevelHistogram)
}

// queryPipelineVersionStageTimeTaken will query the time taken for stages for
// a particular version and a pipeline.
func (es *invocationElasticsearch) queryPipelineVersionStageTimeTaken(ctx context.Context, from, to string, size int, pipelineID string, version int, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	analytics.ApplyCustomEventsEs7(query, filters)

	query = query.Must(es7.NewTermQuery("pipeline_id", pipelineID), es7.NewTermQuery("version", version))

	// Fetch the pipeline ID and extract the version of the pipeline
	// Get pipeline from cache
	pipelineDoc, _ := GetPipelineAndLocFromCache(pipelineID)
	if pipelineDoc == nil {
		return nil, errors.New("couldn't fetch pipeline from cache to extract stage ID's")
	}

	// Get the version of the pipeline
	pipelineVersion, _ := GetPipelineVersion(*pipelineDoc, version)
	if pipelineVersion == nil {
		return nil, errors.New("couldn't fetch pipeline version from cache to extract stage ID's")
	}

	// Unmarshal the pipeline content into a doc.
	var pipelineVersionDoc ESPipelineDoc
	unmarshalErr := json.Unmarshal([]byte(*pipelineVersion.Content), &pipelineVersionDoc)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("error while unmarshaling fetched version of pipeline into ES doc: %s", unmarshalErr.Error())
	}

	// Extract the stage ID's
	stageIds := make([]string, len(*pipelineDoc.Stages))
	for stageIndex, stage := range *pipelineDoc.Stages {
		stageIds[stageIndex] = *stage.ID
	}

	dateHistogramAggr := es7.NewDateHistogramAggregation().Field("timestamp").CalendarInterval("day")
	for _, stageId := range stageIds {
		dateHistogramAggr.SubAggregation(stageId, es7.NewAvgAggregation().Field(fmt.Sprintf("stages.%s.took", stageId)))
	}

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("time_taken_per_month", dateHistogramAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.DateHistogram("time_taken_per_month")
	if !found {
		return nil, fmt.Errorf("error while fetching `time_taken_per_month` for stages avg time taken")
	}

	topLevelHistogram := make([]interface{}, 0)

	for _, dayBucket := range aggrResult.Buckets {
		dayMap := map[string]interface{}{
			"key":           dayBucket.Key,
			"key_as_string": dayBucket.KeyAsString,
		}

		stageArr := make([]interface{}, 0)

		for _, stageId := range stageIds {
			stageResult, found := dayBucket.Aggregations.Avg(stageId)
			if !found {
				return nil, fmt.Errorf("error while fetching stage based avg value")
			}

			stageMap := map[string]interface{}{
				"key":   stageId,
				"value": stageResult.Value,
			}

			stageArr = append(stageArr, stageMap)
		}

		// Append the stage array to the day map
		dayMap["stage_histogram"] = stageArr

		// Append the dayMap to t he topLevelHistogram
		topLevelHistogram = append(topLevelHistogram, dayMap)
	}

	// Finally marshal and return the response
	return json.Marshal(topLevelHistogram)
}

// queryPipelineVersionStageErrorRate will query the error rate for stages for
// a particular pipeline version and a pipeline
func (es *invocationElasticsearch) queryPipelineVersionStageErrorRate(ctx context.Context, from, to string, size int, pipelineID string, version int, filters map[string]interface{}) ([]byte, error) {
	duration := es7.NewRangeQuery("timestamp").
		From(from).
		To(to)

	query := es7.NewBoolQuery().Filter(duration)

	analytics.ApplyCustomEventsEs7(query, filters)

	query = query.Must(es7.NewTermQuery("pipeline_id", pipelineID), es7.NewTermQuery("version", version))

	// Fetch the pipeline ID and extract the version of the pipeline
	// Get pipeline from cache
	pipelineDoc, _ := GetPipelineAndLocFromCache(pipelineID)
	if pipelineDoc == nil {
		return nil, errors.New("couldn't fetch pipeline from cache to extract stage ID's")
	}

	// Get the version of the pipeline
	pipelineVersion, _ := GetPipelineVersion(*pipelineDoc, version)
	if pipelineVersion == nil {
		return nil, errors.New("couldn't fetch pipeline version from cache to extract stage ID's")
	}

	// Unmarshal the pipeline content into a doc.
	var pipelineVersionDoc ESPipelineDoc
	unmarshalErr := json.Unmarshal([]byte(*pipelineVersion.Content), &pipelineVersionDoc)
	if unmarshalErr != nil {
		return nil, fmt.Errorf("error while unmarshaling fetched version of pipeline into ES doc: %s", unmarshalErr.Error())
	}

	// Extract the stage ID's
	stageIds := make([]string, len(*pipelineDoc.Stages))
	for stageIndex, stage := range *pipelineDoc.Stages {
		stageIds[stageIndex] = *stage.ID
	}

	dateHistogramAggr := es7.NewDateHistogramAggregation().Field("timestamp").CalendarInterval("day")
	for _, stageId := range stageIds {
		fieldName := fmt.Sprintf("stages.%s.executed", stageId)
		fieldErrName := fmt.Sprintf("stages.%s.error", stageId)
		mainAggr := es7.NewFilterAggregation().Filter(es7.NewTermQuery(fieldName, true))

		mainAggr.SubAggregation("total", es7.NewValueCountAggregation().Field(fieldName))
		mainAggr.SubAggregation("true", es7.NewFilterAggregation().Filter(es7.NewTermQuery(fieldErrName, true)))

		dateHistogramAggr.SubAggregation(stageId, mainAggr)
	}

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("error_rate_per_month", dateHistogramAggr).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.DateHistogram("error_rate_per_month")
	if !found {
		return nil, fmt.Errorf("error while getting stages error rate per month")
	}

	topLevelHistogram := make([]interface{}, 0)

	for _, dayBucket := range aggrResult.Buckets {
		dayMap := map[string]interface{}{
			"key":           dayBucket.Key,
			"key_as_string": dayBucket.KeyAsString,
		}

		stageArr := make([]interface{}, 0)

		for _, stageId := range stageIds {
			stageIdAggr, found := dayBucket.Aggregations.Filter(stageId)
			if !found {
				return nil, fmt.Errorf("error while getting stage details from aggregations")
			}

			// Extract `total` aggregation
			totalAggr, found := stageIdAggr.Aggregations.ValueCount("total")
			if !found {
				return nil, fmt.Errorf("error while getting the total stage count for stage")
			}

			trueCountAggr, found := stageIdAggr.Aggregations.Filter("true")
			if !found {
				return nil, fmt.Errorf("error while getting the true count for stage")
			}

			errorRate := float64(trueCountAggr.DocCount) / *totalAggr.Value * 100

			// Make sure the value is not NaN
			if math.IsNaN(errorRate) {
				errorRate = 0
			}

			stageArr = append(stageArr, map[string]interface{}{
				"key":   stageId,
				"value": errorRate,
			})
		}

		dayMap["stage_histogram"] = stageArr

		// Finally set the dayMap in the topLevelHistogram
		topLevelHistogram = append(topLevelHistogram, dayMap)
	}

	// Return the marshalled JSON.
	return json.Marshal(topLevelHistogram)
}

// queryPipelineVersionStats will get version stats for each version
func (es *invocationElasticsearch) queryPipelineVersionStats(ctx context.Context, pipelineID string) (map[int]interface{}, error) {
	query := es7.NewTermQuery("pipeline_id", pipelineID)

	termsFilter := es7.NewTermsAggregation().Field("version")
	totalInvocationAggr := es7.NewValueCountAggregation().Field("executed")
	totalTimeTakenAggr := es7.NewAvgAggregation().Field("took")
	errorRateAggr := es7.NewFilterAggregation().Filter(es7.NewTermQuery("error", true))

	termsFilter.SubAggregation("version", totalInvocationAggr).SubAggregation("time_taken", totalTimeTakenAggr).SubAggregation("error_count", errorRateAggr)

	result, err := util.GetClient7().Search(es.indexName).
		Query(query).
		Size(0).
		Aggregation("version_stats", termsFilter).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch pipelines response from es: %v", err)
	}

	aggrResult, found := result.Aggregations.Terms("version_stats")
	if !found {
		return nil, fmt.Errorf("error while fetching version_stats")
	}

	versionStatsMap := make(map[int]interface{})

	for _, versionBucket := range aggrResult.Buckets {
		// Convert the key to integer
		keyAsFloat, isOk := versionBucket.Key.(float64)
		if !isOk {
			return nil, fmt.Errorf("error while converting key to float from interface")
		}
		keyAsInt := int(keyAsFloat)

		// Total invocation count would be the doc_count
		totalInvocationCount := versionBucket.DocCount

		// Extract the count of `error` being true
		errorCountAggr, found := versionBucket.Aggregations.Filter("error_count")
		if !found {
			return nil, fmt.Errorf("error while getting the total error count")
		}
		errorRate := float64(errorCountAggr.DocCount) / float64(totalInvocationCount) * 100
		successRate := 100 - errorRate

		// Extract the avg time taken by versions
		timeTaken, found := versionBucket.Avg("time_taken")
		if !found {
			return nil, fmt.Errorf("error while fetching the avg time taken for versions")
		}

		usageStats := map[string]interface{}{
			"count":        totalInvocationCount,
			"error_rate":   errorRate,
			"success_rate": successRate,
			"took":         timeTaken.Value,
		}

		versionStatsMap[keyAsInt] = usageStats
	}

	return versionStatsMap, nil
}
