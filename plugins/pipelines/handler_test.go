package pipelines

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	. "github.com/smartystreets/goconvey/convey"
)

func getStageToMockRSAPIResponse() ESPipelineStage {
	id := "mock_rsapi_response"
	script := "function handleRequest() { return { response: { body: JSON.stringify({'settings':{'took':1,'script_took':0},'search':{'took':1,'timed_out':false,'_shards':{'total':1,'successful':1,'skipped':0,'failed':0},'hits':{'total':{'value':1,'relation':'eq'},'max_score':1,'hits':[{'_index':'test','_type':'_doc','_id':'1','_score':1,'_source':{'queryLength':6,'query':'value1'}}]},'status':200}}), code: 200} } }"
	return ESPipelineStage{
		ID:     &id,
		Script: &script,
	}
}
func TestExecutePipeline(t *testing.T) {
	Convey("sync script", t, func() {
		stageID := "modify-request"
		script := "function handleRequest() { return { request: { body: JSON.stringify({ query: [{ 'id': 'search' }] }), headers: { auth: 'JdeF=='}} }; }"
		stages := []ESPipelineStage{
			{
				ID:     &stageID,
				Script: &script,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\"}]}\",\"headers\":{\"auth\":\"JdeF==\"}},\"response\":{\"body\":\"\",\"code\":200,\"headers\":{}}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("PromoteResults pre-built stage", t, func() {
		use := PromoteResults
		promotedResults := []interface{}{
			map[string]interface{}{
				"doc": map[string]interface{}{
					"_id":     "id_1",
					"_source": map[string]interface{}{},
				},
				"position": 10,
			},
			map[string]interface{}{
				"doc": map[string]interface{}{
					"_id":     "id_2",
					"_source": map[string]interface{}{},
				},
				"position": 3,
			},
		}
		inputs := map[string]interface{}{
			"data": promotedResults,
		}

		inputsInBytes, _ := json.Marshal(inputs)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			getStageToMockRSAPIResponse(),
			{
				Use:    &use,
				Inputs: &inputsInStrings,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		queryId := "search"
		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID: &queryId,
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body: requestBodyInBytes,
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\"}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":1,\\\"script_took\\\":0},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[{\\\"_index\\\":\\\"test\\\",\\\"_type\\\":\\\"_doc\\\",\\\"_id\\\":\\\"1\\\",\\\"_score\\\":1,\\\"_source\\\":{\\\"queryLength\\\":6,\\\"query\\\":\\\"value1\\\"}}]},\\\"status\\\":200,\\\"promoted\\\":[{\\\"doc\\\":{\\\"_id\\\":\\\"id_1\\\",\\\"_source\\\":{}},\\\"position\\\":10},{\\\"doc\\\":{\\\"_id\\\":\\\"id_2\\\",\\\"_source\\\":{}},\\\"position\\\":3}]}}\",\"code\":200,\"headers\":null}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("HideResults pre-built stage", t, func() {
		use := HideResults
		hiddenResults := []interface{}{
			"1",
		}
		inputs := map[string]interface{}{
			"data": hiddenResults,
		}

		inputsInBytes, _ := json.Marshal(inputs)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			getStageToMockRSAPIResponse(),
			{
				Use:    &use,
				Inputs: &inputsInStrings,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		queryId := "search"
		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID: &queryId,
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body: requestBodyInBytes,
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\"}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":1,\\\"script_took\\\":0},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[]},\\\"status\\\":200,\\\"hidden\\\":1}}\",\"code\":200,\"headers\":null}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("CustomData pre-built stage", t, func() {
		use := CustomData
		customData := []interface{}{
			map[string]interface{}{
				"doc": map[string]interface{}{
					"_id":     "id_12",
					"_source": map[string]interface{}{},
				},
				"position": 12,
			},
		}
		inputs := map[string]interface{}{
			"data": customData,
		}

		inputsInBytes, _ := json.Marshal(inputs)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			getStageToMockRSAPIResponse(),
			{
				Use:    &use,
				Inputs: &inputsInStrings,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		queryId := "search"
		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID: &queryId,
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body: requestBodyInBytes,
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\"}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":1,\\\"script_took\\\":0},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[{\\\"_index\\\":\\\"test\\\",\\\"_type\\\":\\\"_doc\\\",\\\"_id\\\":\\\"1\\\",\\\"_score\\\":1,\\\"_source\\\":{\\\"queryLength\\\":6,\\\"query\\\":\\\"value1\\\"}}]},\\\"status\\\":200,\\\"customData\\\":[{\\\"doc\\\":{\\\"_id\\\":\\\"id_12\\\",\\\"_source\\\":{}},\\\"position\\\":12}]}}\",\"code\":200,\"headers\":null}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("ReplaceSearchTerm pre-built stage", t, func() {
		use := ReplaceSearchTerm
		var replaceTerm = "iphoneX"

		inputs := map[string]interface{}{
			"data": replaceTerm,
		}

		inputsInBytes, _ := json.Marshal(inputs)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			getStageToMockRSAPIResponse(),
			{
				Use:    &use,
				Inputs: &inputsInStrings,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		queryId := "search"
		var testValue interface{} = "someData"

		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID:    &queryId,
					Type:  querytranslate.Search,
					Value: &testValue,
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body: requestBodyInBytes,
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\",\\\"value\\\":\\\"\\\\\\\"iphoneX\\\\\\\"\\\"}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":1,\\\"script_took\\\":0},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[{\\\"_index\\\":\\\"test\\\",\\\"_type\\\":\\\"_doc\\\",\\\"_id\\\":\\\"1\\\",\\\"_score\\\":1,\\\"_source\\\":{\\\"queryLength\\\":6,\\\"query\\\":\\\"value1\\\"}}]},\\\"status\\\":200}}\",\"code\":200,\"headers\":null}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("AddFilter pre-built stage", t, func() {
		use := AddFilter
		var filterTerm = map[string]interface{}{
			"year": "2011",
		}

		inputs := map[string]interface{}{
			"data": filterTerm,
		}

		inputsInBytes, _ := json.Marshal(inputs)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			getStageToMockRSAPIResponse(),
			{
				Use:    &use,
				Inputs: &inputsInStrings,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		queryId := "search"
		var testValue interface{} = "someData"

		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID:    &queryId,
					Type:  querytranslate.Search,
					Value: &testValue,
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body: requestBodyInBytes,
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\",\\\"react\\\":{\\\"and\\\":\\\"query_rule_filter_year\\\"},\\\"value\\\":\\\"someData\\\"},{\\\"id\\\":\\\"query_rule_filter_year\\\",\\\"type\\\":\\\"term\\\",\\\"dataField\\\":[\\\"year\\\"],\\\"value\\\":\\\"2011\\\",\\\"execute\\\":false}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":1,\\\"script_took\\\":0},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[{\\\"_index\\\":\\\"test\\\",\\\"_type\\\":\\\"_doc\\\",\\\"_id\\\":\\\"1\\\",\\\"_score\\\":1,\\\"_source\\\":{\\\"queryLength\\\":6,\\\"query\\\":\\\"value1\\\"}}]},\\\"status\\\":200}}\",\"code\":200,\"headers\":null}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("RemoveWords pre-built stage", t, func() {
		use := RemoveWords
		var removeWords = []interface{}{
			"iphone5s", "iphone5",
		}

		inputs := map[string]interface{}{
			"data": removeWords,
		}

		inputsInBytes, _ := json.Marshal(inputs)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			getStageToMockRSAPIResponse(),
			{
				Use:    &use,
				Inputs: &inputsInStrings,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		queryId := "search"
		var testValue interface{} = "some iphone5s are better than iphone5 and iphoneX"

		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID:    &queryId,
					Type:  querytranslate.Search,
					Value: &testValue,
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body: requestBodyInBytes,
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\",\\\"value\\\":\\\"some  are better than  and iphoneX\\\"}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":1,\\\"script_took\\\":0},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[{\\\"_index\\\":\\\"test\\\",\\\"_type\\\":\\\"_doc\\\",\\\"_id\\\":\\\"1\\\",\\\"_score\\\":1,\\\"_source\\\":{\\\"queryLength\\\":6,\\\"query\\\":\\\"value1\\\"}}]},\\\"status\\\":200}}\",\"code\":200,\"headers\":null}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("ReplaceWords pre-built stage", t, func() {
		use := ReplaceWords
		var replaceWords = map[string]interface{}{
			"iphone": "iphoneX",
			"batman": "batman movie",
		}

		inputs := map[string]interface{}{
			"data": replaceWords,
		}

		inputsInBytes, _ := json.Marshal(inputs)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			getStageToMockRSAPIResponse(),
			{
				Use:    &use,
				Inputs: &inputsInStrings,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}
		queryId := "search"
		var testValue interface{} = "some batmans better than iphone5 and iphone"

		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID:    &queryId,
					Type:  querytranslate.Search,
					Value: &testValue,
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body: requestBodyInBytes,
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\",\\\"value\\\":\\\"some batman movies better than iphoneX5 and iphoneX\\\"}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":1,\\\"script_took\\\":0},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[{\\\"_index\\\":\\\"test\\\",\\\"_type\\\":\\\"_doc\\\",\\\"_id\\\":\\\"1\\\",\\\"_score\\\":1,\\\"_source\\\":{\\\"queryLength\\\":6,\\\"query\\\":\\\"value1\\\"}}]},\\\"status\\\":200}}\",\"code\":200,\"headers\":null}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("caching", t, func() {
		modifyRequest := "modify_request"
		modifyRequestScript := "function handleRequest() { const requestBody = JSON.parse(context.request.body); return { request: {...context.request, body: JSON.stringify({...requestBody, query: [...requestBody.query, {'id': 'search2'}]})}}}"

		useCacheStage := UseCache

		True := true

		stages := []ESPipelineStage{
			{
				ID:              &modifyRequest,
				Script:          &modifyRequestScript,
				ContinueOnError: &True,
			},
			// apply cache after request modifications
			{
				Use: &useCacheStage,
			},
			// mock ES Response
			getStageToMockRSAPIResponse(),
		}
		PipelineId := "cache_test"
		pipeline := ESPipelineDoc{
			ID:     &PipelineId,
			Stages: &stages,
		}
		queryId := "search"
		requestBody := querytranslate.RSQuery{
			Query: []querytranslate.Query{
				{
					ID:        &queryId,
					DataField: "ded",
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)
		pipelineExecutionContext := PipelineExecutionContext{
			envs: map[string]interface{}{
				"category": "reactivesearch",
				"index":    []interface{}{"test"},
				"path":     "/test/_reactivesearch",
			},
			request: PipelineExecutionRequest{Body: requestBodyInBytes},
		}
		// make request to cache request
		pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		// resInBytes, _ := json.Marshal(res)
		start := time.Now()
		// wait for 2 seconds because cache recording happens in a go routine
		var cachedRes map[string]interface{}
		for {
			if time.Since(start) > 2*time.Second {
				// make another request, the response should be cached
				cachedRes, _ = pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
				break
			}
		}
		cachedResInBytes, _ := json.Marshal(cachedRes)
		expectedResponse := "{\"console_logs\":[],\"envs\":{\"category\":\"reactivesearch\",\"index\":[\"test\"],\"path\":\"/test/_reactivesearch\"},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"search\\\",\\\"dataField\\\":\\\"ded\\\"},{\\\"id\\\":\\\"search2\\\"}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"{\\\"settings\\\":{\\\"took\\\":0,\\\"script_took\\\":0,\\\"cached\\\":true},\\\"search\\\":{\\\"took\\\":1,\\\"timed_out\\\":false,\\\"_shards\\\":{\\\"total\\\":1,\\\"successful\\\":1,\\\"skipped\\\":0,\\\"failed\\\":0},\\\"hits\\\":{\\\"total\\\":{\\\"value\\\":1,\\\"relation\\\":\\\"eq\\\"},\\\"max_score\\\":1,\\\"hits\\\":[{\\\"_index\\\":\\\"test\\\",\\\"_type\\\":\\\"_doc\\\",\\\"_id\\\":\\\"1\\\",\\\"_score\\\":1,\\\"_source\\\":{\\\"queryLength\\\":6,\\\"query\\\":\\\"value1\\\"}}]},\\\"status\\\":200}}\",\"code\":200,\"headers\":{\"X-request-Cache\":\"true\",\"x-pipeline-id\":\"cache_test\"}}}"
		So(string(cachedResInBytes), ShouldResemble, expectedResponse)
	})

	Convey("searchrelevancy stage", t, func() {
		searchRelevancyStage := SearchRelevancy

		searchRelevancyInputMap := map[string]interface{}{
			"search": map[string]interface{}{
				"dataField": []string{"original_title"},
				"size":      10,
			},
			"term": map[string]interface{}{
				"dataField":       "authors",
				"aggregationSize": 5,
			},
			"range": map[string]interface{}{
				"dataField":         "price",
				"includeNullValues": true,
			},
			"geo": map[string]interface{}{
				"defaultQuery": map[string]interface{}{
					"query": map[string]interface{}{
						"geo": map[string]interface{}{
							"field": "location",
						},
					},
				},
			},
			"suggestion": map[string]interface{}{
				"dataField":                []string{"original_title"},
				"size":                     4,
				"enablePopularSuggestions": true,
				"popularSuggestionsConfig": map[string]interface{}{
					"size": 2,
				},
				"enableRecentSuggestions": true,
				"recentSuggestionsConfig": map[string]interface{}{
					"size": 2,
				},
				"urlField":      "url",
				"categoryField": "authors",
			},
		}
		False := false

		inputsInBytes, _ := json.Marshal(searchRelevancyInputMap)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			{
				Use:             &searchRelevancyStage,
				Inputs:          &inputsInStrings,
				ContinueOnError: &False,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}

		requestBody := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":   "query-1",
					"type": "search",
				},
				{
					"id":   "query-2",
					"type": "suggestion",
				},
				{
					"id":   "query-3",
					"type": "geo",
				},
				{
					"id":   "query-4",
					"type": "term",
				},
				{
					"id":   "query-5",
					"type": "range",
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)

		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body:    requestBodyInBytes,
				Headers: make(map[string]string),
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{},\"request\":{\"body\":\"{\\\"query\\\":[{\\\"id\\\":\\\"query-1\\\",\\\"dataField\\\":[\\\"original_title\\\"],\\\"size\\\":10},{\\\"id\\\":\\\"query-2\\\",\\\"type\\\":\\\"suggestion\\\",\\\"dataField\\\":[\\\"original_title\\\"],\\\"categoryField\\\":\\\"authors\\\",\\\"size\\\":4,\\\"enableRecentSuggestions\\\":true,\\\"recentSuggestionsConfig\\\":{\\\"size\\\":2},\\\"enablePopularSuggestions\\\":true,\\\"popularSuggestionsConfig\\\":{\\\"size\\\":2},\\\"urlField\\\":\\\"url\\\"},{\\\"id\\\":\\\"query-3\\\",\\\"type\\\":\\\"geo\\\",\\\"defaultQuery\\\":{\\\"query\\\":{\\\"geo\\\":{\\\"field\\\":\\\"location\\\"}}}},{\\\"id\\\":\\\"query-4\\\",\\\"type\\\":\\\"term\\\",\\\"dataField\\\":\\\"authors\\\",\\\"aggregationSize\\\":5},{\\\"id\\\":\\\"query-5\\\",\\\"type\\\":\\\"range\\\",\\\"dataField\\\":\\\"price\\\",\\\"includeNullValues\\\":true}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"\",\"code\":200,\"headers\":{}}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})

	Convey("reactivesearch query stage: mongodb", t, func() {
		rsQueryStage := ReactiveSearchQuery

		rsQueryInputsMap := map[string]interface{}{
			"backend": "mongodb",
		}
		False := false

		inputsInBytes, _ := json.Marshal(rsQueryInputsMap)
		inputsInStrings := string(inputsInBytes)

		stages := []ESPipelineStage{
			{
				Use:             &rsQueryStage,
				Inputs:          &inputsInStrings,
				ContinueOnError: &False,
			},
		}
		pipeline := ESPipelineDoc{
			Stages: &stages,
		}

		requestBody := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":   "query-1",
					"type": "search",
				},
			},
		}
		requestBodyInBytes, _ := json.Marshal(requestBody)

		pipelineExecutionContext := PipelineExecutionContext{
			envs: make(map[string]interface{}),
			request: PipelineExecutionRequest{
				Body:    requestBodyInBytes,
				Headers: make(map[string]string),
			},
		}
		res, _ := pipeline.executePipeline(pipelineExecutionContext, nil, false, nil, false, false, false, nil)
		resInBytes, _ := json.Marshal(res)

		expectedResponse := "{\"console_logs\":[],\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"id\\\":\\\"query-1\\\",\\\"type\\\":\\\"search\\\"}]}\",\"RS_BACKEND\":\"mongodb\"},\"request\":{\"body\":\"{\\\"query-1\\\":[{\\\"$facet\\\":{\\\"hits\\\":[{\\\"$limit\\\":10}],\\\"total\\\":[{\\\"$count\\\":\\\"count\\\"}]}}]}\",\"headers\":{},\"method\":\"\",\"url\":\"\"},\"response\":{\"body\":\"\",\"code\":200,\"headers\":{}}}"
		So(string(resInBytes), ShouldResemble, expectedResponse)
	})
}

func TestGetInputValuesFromContext(t *testing.T) {
	Convey("static input values", t, func() {
		inputMap := map[string]interface{}{
			"key1": "value1",
			"key2": true,
		}
		inputsInBytes, _ := json.Marshal(inputMap)
		inputAsString := string(inputsInBytes)
		resolvedInputs, _ := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"a": 1,
		})

		responseInBytes, _ := json.Marshal(map[string]interface{}{
			"key1": "value1",
			"key2": true,
		})
		responseAsString := string(responseInBytes)

		So(resolvedInputs, ShouldResemble, &responseAsString)
	})
	Convey("Dynamic input values", t, func() {
		inputsContext := map[string]interface{}{
			"key1": "x/{{value1}}",
			"key2": "{{value2}}",
		}
		inputsContextInBytes, _ := json.Marshal(inputsContext)

		inputAsString := string(inputsContextInBytes)

		resolvedInputs, _ := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"value1": "val1",
			"value2": true,
		})

		responseInBytes, _ := json.Marshal(map[string]interface{}{
			"key1": "x/val1",
			"key2": "true",
		})
		responseAsString := string(responseInBytes)

		So(resolvedInputs, ShouldResemble, &responseAsString)
	})
	Convey("Dynamic input value multiple nests", t, func() {
		inputsContext := map[string]interface{}{
			"key1": "x/{{value1.value2obj.value3}}",
			"key2": "{{value2}}",
		}
		inputsContextInBytes, _ := json.Marshal(inputsContext)

		inputAsString := string(inputsContextInBytes)

		resolvedInputs, _ := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"value1": map[string]interface{}{
				"value2obj": map[string]interface{}{
					"value3": "someVal",
				},
			},
			"value2": true,
		})

		responseInBytes, _ := json.Marshal(map[string]interface{}{
			"key1": "x/someVal",
			"key2": "true",
		})
		responseAsString := string(responseInBytes)

		So(resolvedInputs, ShouldResemble, &responseAsString)
	})
	Convey("Dynamic input value multiple nests with type preserve", t, func() {
		inputsContext := map[string]interface{}{
			"key1": "x/{{{value1.value2obj.value3}}}",
			"key2": "{{value2}}",
		}
		inputsContextInBytes, _ := json.Marshal(inputsContext)

		inputAsString := string(inputsContextInBytes)

		resolvedInputs, _ := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"value1": map[string]interface{}{
				"value2obj": map[string]interface{}{
					"value3": "someVal",
				},
			},
			"value2": true,
		})

		responseAsString := "{\"key1\":\"x/\"someVal\"\",\"key2\":\"true\"}"

		So(resolvedInputs, ShouldResemble, &responseAsString)
	})
	Convey("Dynamic input value multiple nests with type preserve", t, func() {
		inputsContext := map[string]interface{}{
			"key1": "x/{{value1.value2obj.value3}}",
			"key2": "{{{value2}}}",
		}
		inputsContextInBytes, _ := json.Marshal(inputsContext)

		inputAsString := string(inputsContextInBytes)

		resolvedInputs, _ := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"value1": map[string]interface{}{
				"value2obj": map[string]interface{}{
					"value3": true,
				},
			},
			"value2": true,
		})

		responseInBytes, _ := json.Marshal(map[string]interface{}{
			"key1": "x/true",
			"key2": true,
		})
		responseAsString := string(responseInBytes)

		So(resolvedInputs, ShouldResemble, &responseAsString)
	})
	Convey("Dynamic input value failure", t, func() {
		inputsContext := map[string]interface{}{
			"key1": "x/{{value1.value2obj.value3}}",
			"key2": "{{value2}}",
		}
		inputsContextInBytes, _ := json.Marshal(inputsContext)

		inputAsString := string(inputsContextInBytes)

		_, inputErr := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"value1": map[string]interface{}{
				"value2obj": "someVal",
			},
			"value2": true,
		})

		expectedError := errors.New("`value2obj` is not an object, incorrect syntax passed for substitution!")

		So(inputErr, ShouldResemble, expectedError)
	})
	Convey("Dynamic input value failure", t, func() {
		inputsContext := map[string]interface{}{
			"key1": "x/{{value1.value2obj.value3}}",
			"key2": "{{value2}}",
		}
		inputsContextInBytes, _ := json.Marshal(inputsContext)

		inputAsString := string(inputsContextInBytes)

		_, inputErr := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"value1": map[string]interface{}{
				"value2obj": map[string]interface{}{
					"valueNone": 3,
				},
			},
			"value2": true,
		})

		expectedError := errors.New("no `value3` key present inside `value1.value2obj`")

		So(inputErr, ShouldResemble, expectedError)
	})
	Convey("Replace with type preserved", t, func() {
		inputsContext := map[string]interface{}{
			"key2": "{{{value2}}}",
		}
		inputsContextInBytes, _ := json.Marshal(inputsContext)

		inputAsString := string(inputsContextInBytes)

		resolvedInputs, _ := getInputValuesFromContext(&inputAsString, map[string]interface{}{
			"value2": true,
		})

		responseInBytes, _ := json.Marshal(map[string]interface{}{
			"key2": true,
		})
		responseAsString := string(responseInBytes)

		So(resolvedInputs, ShouldResemble, &responseAsString)
	})
}
