package pipelines

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	. "github.com/smartystreets/goconvey/convey"
)

func runTestStep(requestBody map[string]interface{}) ([]byte, bool, *Error) {
	rsQueryStage := ReactiveSearchQuery
	rsStage := ESPipelineStage{
		Use: &rsQueryStage,
	}

	stageInputs := map[string]interface{}{
		"backend": "solr",
	}
	stageInputsInBytes, _ := json.Marshal(stageInputs)
	stageInputsAsString := string(stageInputsInBytes)

	scriptContext := ExecutePipelineResponse{
		Request: rules.ScriptRequest{},
		// Default response to write.
		// It is the responsibility of stage to write the response if
		// ElasticsearchQuery stage is not present
		Response: rules.ScriptResponse{
			Code:    http.StatusOK,
			Body:    "",
			Headers: make(map[string]string),
		},
		Environments: make(map[string]interface{}),
		Logs:         make(map[string][]string),
	}

	rsAPIRequest := ReactiveSearchQueryContext{}
	scriptEnvs := make(map[string]interface{})
	isAsync := false

	// Time is not necessary while testing
	startTime := time.Now()

	bodyMarshalled, _ := json.Marshal(requestBody)
	scriptContext.Request.Body = string(bodyMarshalled)

	scriptContextBytes, _ := json.Marshal(scriptContext)

	globalScriptContext := GlobalScriptContext{
		value: scriptContextBytes,
	}

	return executeReactivesearchStage(rsStage, &stageInputsAsString, &globalScriptContext, &rsAPIRequest, scriptEnvs, isAsync, &startTime, false)
}

func TestSolrTermQuery(t *testing.T) {
	metadata := map[string]interface{}{
		"app":            "appbase",
		"profile":        "appbase",
		"search_profile": "appbase",
	}

	Convey("solr term: queryFormat as `AND`", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":          "test",
					"value":       []interface{}{"quark", "something"},
					"dataField":   "summary_t",
					"type":        "term",
					"queryFormat": "and",
					"execute":     false,
				},
				map[string]interface{}{
					"id": "second",
					"react": map[string]interface{}{
						"and": "test",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"test\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"(summary_t:\\\\\\\"quark AND something\\\\\\\")\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"execute\\\":false,\\\"id\\\":\\\"test\\\",\\\"queryFormat\\\":\\\"and\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":[\\\"quark\\\",\\\"something\\\"]},{\\\"id\\\":\\\"second\\\",\\\"react\\\":{\\\"and\\\":\\\"test\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr term: queryFormat as `OR`", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":          "test",
					"value":       []interface{}{"quark", "something"},
					"dataField":   "summary_t",
					"type":        "term",
					"queryFormat": "or",
					"execute":     false,
				},
				map[string]interface{}{
					"id": "second",
					"react": map[string]interface{}{
						"and": "test",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"test\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"(summary_t:\\\\\\\"quark OR something\\\\\\\")\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"execute\\\":false,\\\"id\\\":\\\"test\\\",\\\"queryFormat\\\":\\\"or\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":[\\\"quark\\\",\\\"something\\\"]},{\\\"id\\\":\\\"second\\\",\\\"react\\\":{\\\"and\\\":\\\"test\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr term: pivot facets", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":          "test",
					"value":       []interface{}{"quark > something", "test > something"},
					"dataField":   []interface{}{"summary_t", "description_t"},
					"type":        "term",
					"queryFormat": "or",
					"execute":     false,
				},
				map[string]interface{}{
					"id": "second",
					"react": map[string]interface{}{
						"and": "test",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"test\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"((summary_t:\\\\\\\"quark\\\\\\\") AND (description_t:\\\\\\\"something\\\\\\\")) OR ((summary_t:\\\\\\\"test\\\\\\\") AND (description_t:\\\\\\\"something\\\\\\\"))\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"dataField\\\":[\\\"summary_t\\\",\\\"description_t\\\"],\\\"execute\\\":false,\\\"id\\\":\\\"test\\\",\\\"queryFormat\\\":\\\"or\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":[\\\"quark \\\\u003e something\\\",\\\"test \\\\u003e something\\\"]},{\\\"id\\\":\\\"second\\\",\\\"react\\\":{\\\"and\\\":\\\"test\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr term: pivot facets with more values than dataFields", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":          "test",
					"value":       []interface{}{"quark > something", "test > something", "three > word > value"},
					"dataField":   []interface{}{"summary_t", "description_t"},
					"type":        "term",
					"queryFormat": "and",
					"execute":     false,
				},
				map[string]interface{}{
					"id": "second",
					"react": map[string]interface{}{
						"and": "test",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"test\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"((summary_t:\\\\\\\"quark\\\\\\\") AND (description_t:\\\\\\\"something\\\\\\\")) AND ((summary_t:\\\\\\\"test\\\\\\\") AND (description_t:\\\\\\\"something\\\\\\\")) AND ((summary_t:\\\\\\\"three\\\\\\\") AND (description_t:\\\\\\\"word\\\\\\\"))\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"dataField\\\":[\\\"summary_t\\\",\\\"description_t\\\"],\\\"execute\\\":false,\\\"id\\\":\\\"test\\\",\\\"queryFormat\\\":\\\"and\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":[\\\"quark \\\\u003e something\\\",\\\"test \\\\u003e something\\\",\\\"three \\\\u003e word \\\\u003e value\\\"]},{\\\"id\\\":\\\"second\\\",\\\"react\\\":{\\\"and\\\":\\\"test\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr term: normal react query", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":          "test",
					"value":       "quark",
					"dataField":   "summary_t",
					"type":        "term",
					"queryFormat": "and",
					"execute":     true,
				},
				map[string]interface{}{
					"id": "second",
					"react": map[string]interface{}{
						"and": "test",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"test\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"(summary_t:\\\\\\\"quark\\\\\\\")\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"},\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"term\\\\\\\",\\\\\\\"queryFormat\\\\\\\":\\\\\\\"and\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\",\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet\\\":\\\"true\\\",\\\"facet.field\\\":\\\"summary_t\\\",\\\"facet.sort\\\":\\\"count\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark\\\",\\\"rows\\\":\\\"0\\\",\\\"sort\\\":\\\"id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"execute\\\":true,\\\"id\\\":\\\"test\\\",\\\"queryFormat\\\":\\\"and\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":\\\"quark\\\"},{\\\"id\\\":\\\"second\\\",\\\"react\\\":{\\\"and\\\":\\\"test\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr term: defaultQuery", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":    "test",
					"value": "quark",
					"type":  "term",
					"defaultQuery": map[string]interface{}{
						"query": "?facet=true&facet.field=summary_t&fl=score%2C*&hl=false&hl.fl=*&q=quark&qf=summary_t&rows=0&sort=score%20desc&start=0",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"term\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\",\\\\\\\"defaultQuery\\\\\\\":{\\\\\\\"query\\\\\\\":\\\\\\\"?facet=true\\\\\\\\u0026facet.field=summary_t\\\\\\\\u0026fl=score%2C*\\\\\\\\u0026hl=false\\\\\\\\u0026hl.fl=*\\\\\\\\u0026q=quark\\\\\\\\u0026qf=summary_t\\\\\\\\u0026rows=0\\\\\\\\u0026sort=score%20desc\\\\\\\\u0026start=0\\\\\\\"}}\\\",\\\"facet\\\":\\\"true\\\",\\\"facet.field\\\":\\\"summary_t\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark\\\",\\\"qf\\\":\\\"summary_t\\\",\\\"rows\\\":\\\"0\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"defaultQuery\\\":{\\\"query\\\":\\\"?facet=true\\\\u0026facet.field=summary_t\\\\u0026fl=score%2C*\\\\u0026hl=false\\\\u0026hl.fl=*\\\\u0026q=quark\\\\u0026qf=summary_t\\\\u0026rows=0\\\\u0026sort=score%20desc\\\\u0026start=0\\\"},\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr term: customQuery", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":    "test",
					"value": "quark",
					"type":  "term",
					"customQuery": map[string]interface{}{
						"query": "?facet=true&facet.field=summary_t&fq=((summary_t%3Aquark))&fl=score%2C*&hl=false&hl.fl=*&q=quark&qf=summary_t&rows=0&sort=score%20desc&start=0",
					},
				},
				map[string]interface{}{
					"id": "second",
					"react": map[string]interface{}{
						"or": "test",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"or\\\\\\\":\\\\\\\"test\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"((summary_t:quark))\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"},\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"term\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\",\\\\\\\"customQuery\\\\\\\":{\\\\\\\"query\\\\\\\":\\\\\\\"?facet=true\\\\\\\\u0026facet.field=summary_t\\\\\\\\u0026fq=((summary_t%3Aquark))\\\\\\\\u0026fl=score%2C*\\\\\\\\u0026hl=false\\\\\\\\u0026hl.fl=*\\\\\\\\u0026q=quark\\\\\\\\u0026qf=summary_t\\\\\\\\u0026rows=0\\\\\\\\u0026sort=score%20desc\\\\\\\\u0026start=0\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet\\\":\\\"true\\\",\\\"facet.field\\\":\\\"\\\",\\\"facet.sort\\\":\\\"count\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark\\\",\\\"rows\\\":\\\"0\\\",\\\"sort\\\":\\\"id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"customQuery\\\":{\\\"query\\\":\\\"?facet=true\\\\u0026facet.field=summary_t\\\\u0026fq=((summary_t%3Aquark))\\\\u0026fl=score%2C*\\\\u0026hl=false\\\\u0026hl.fl=*\\\\u0026q=quark\\\\u0026qf=summary_t\\\\u0026rows=0\\\\u0026sort=score%20desc\\\\u0026start=0\\\"},\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":\\\"quark\\\"},{\\\"id\\\":\\\"second\\\",\\\"react\\\":{\\\"or\\\":\\\"test\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr term: skip q and qf since no hits are required", t, func() {
		requestBody := map[string]interface{}{
			"metadata": metadata,
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"value":     "quark",
					"dataField": "summary_t",
					"type":      "term",
					"sortBy":    "count",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"term\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\",\\\\\\\"sortBy\\\\\\\":\\\\\\\"count\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet\\\":\\\"true\\\",\\\"facet.field\\\":\\\"summary_t\\\",\\\"facet.sort\\\":\\\"count\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark\\\",\\\"rows\\\":\\\"0\\\",\\\"sort\\\":\\\"id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"metadata\\\":{\\\"app\\\":\\\"appbase\\\",\\\"profile\\\":\\\"appbase\\\",\\\"search_profile\\\":\\\"appbase\\\"},\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"id\\\":\\\"test\\\",\\\"sortBy\\\":\\\"count\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})
}

func TestSolrSearchQuery(t *testing.T) {
	Convey("solr search: basic search translation", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"value":     "quark",
					"dataField": "summary_t",
					"type":      "search",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark*\\\",\\\"qf\\\":\\\"summary_t\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"search\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: id and dataField", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"dataField": "summary_t",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"id\\\":\\\"test\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: id, value and dataField", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"value":     "quark",
					"dataField": "summary_t",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark*\\\",\\\"qf\\\":\\\"summary_t\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"id\\\":\\\"test\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: id only", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id": "test",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"id\\\":\\\"test\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: id, value and react", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":    "test",
					"value": "quark",
					"react": map[string]interface{}{
						"and": "second",
					},
				},
				map[string]interface{}{
					"id":        "second",
					"dataField": "summary_t",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"},\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"second\\\\\\\"},\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"(summary_t:*)\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark*\\\",\\\"qf\\\":\\\"\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"id\\\":\\\"test\\\",\\\"react\\\":{\\\"and\\\":\\\"second\\\"},\\\"value\\\":\\\"quark\\\"},{\\\"dataField\\\":\\\"summary_t\\\",\\\"id\\\":\\\"second\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: sortBy desc", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":     "test",
					"value":  "quark",
					"sortBy": "desc",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"sortBy\\\\\\\":\\\\\\\"desc\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark*\\\",\\\"qf\\\":\\\"\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"id\\\":\\\"test\\\",\\\"sortBy\\\":\\\"desc\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: sortBy asc", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":     "test",
					"value":  "quark",
					"sortBy": "asc",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"sortBy\\\\\\\":\\\\\\\"asc\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark*\\\",\\\"qf\\\":\\\"\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"id\\\":\\\"test\\\",\\\"sortBy\\\":\\\"asc\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"
		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: qf should not be present if q is not or is *", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "result",
					"type": "search",
					"dataField": []interface{}{
						"published_dt",
					},
					"includeFields": []interface{}{"title_t", "published_dt"},
					"execute":       true,
					"react": map[string]interface{}{
						"and": []interface{}{
							"filter_by_product",
							"search",
							"Authors_0",
							"Categories_1",
							"Published_date_2",
							"ToggleResults",
							"result__internal",
						},
					},
					"highlight": true,
					"size":      9,
					"sortBy":    "desc",
				},
				map[string]interface{}{
					"id":      "search",
					"type":    "search",
					"execute": false,
					"react": map[string]interface{}{
						"and": "search__internal",
					},
					"highlight": false,
					"size":      6,
					"value":     "photon production",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"result\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"result\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":[\\\\\\\"filter_by_product\\\\\\\",\\\\\\\"search\\\\\\\",\\\\\\\"Authors_0\\\\\\\",\\\\\\\"Categories_1\\\\\\\",\\\\\\\"Published_date_2\\\\\\\",\\\\\\\"ToggleResults\\\\\\\",\\\\\\\"result__internal\\\\\\\"]},\\\\\\\"dataField\\\\\\\":[\\\\\\\"published_dt\\\\\\\"],\\\\\\\"size\\\\\\\":9,\\\\\\\"sortBy\\\\\\\":\\\\\\\"desc\\\\\\\",\\\\\\\"includeFields\\\\\\\":[\\\\\\\"title_t\\\\\\\",\\\\\\\"published_dt\\\\\\\"],\\\\\\\"highlight\\\\\\\":true,\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"title_t,published_dt,score,id\\\",\\\"hl\\\":\\\"true\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"photon OR production*\\\",\\\"rows\\\":\\\"9\\\",\\\"sort\\\":\\\"published_dt desc, id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"published_dt\\\"],\\\"execute\\\":true,\\\"highlight\\\":true,\\\"id\\\":\\\"result\\\",\\\"includeFields\\\":[\\\"title_t\\\",\\\"published_dt\\\"],\\\"react\\\":{\\\"and\\\":[\\\"filter_by_product\\\",\\\"search\\\",\\\"Authors_0\\\",\\\"Categories_1\\\",\\\"Published_date_2\\\",\\\"ToggleResults\\\",\\\"result__internal\\\"]},\\\"size\\\":9,\\\"sortBy\\\":\\\"desc\\\",\\\"type\\\":\\\"search\\\"},{\\\"execute\\\":false,\\\"highlight\\\":false,\\\"id\\\":\\\"search\\\",\\\"react\\\":{\\\"and\\\":\\\"search__internal\\\"},\\\"size\\\":6,\\\"type\\\":\\\"search\\\",\\\"value\\\":\\\"photon production\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: wrap value in quotes if string", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "Authors_0",
					"type": "term",
					"dataField": []interface{}{
						"authors_ss",
					},
					"execute":        true,
					"size":           5,
					"sortBy":         "count",
					"showMissing":    false,
					"value":          []interface{}{"D. Zeppenfeld"},
					"selectAllLabel": nil,
				},
				map[string]interface{}{
					"id":   "result",
					"type": "search",
					"dataField": []interface{}{
						"_score",
					},
					"execute": true,
					"react": map[string]interface{}{
						"and": []interface{}{
							"Authors_0",
						},
					},
					"highlight": true,
					"size":      9,
					"sortBy":    "desc",
					"defaultQuery": map[string]interface{}{
						"track_total_hits": true,
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"Authors_0\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"Authors_0\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"term\\\\\\\",\\\\\\\"dataField\\\\\\\":[\\\\\\\"authors_ss\\\\\\\"],\\\\\\\"size\\\\\\\":5,\\\\\\\"sortBy\\\\\\\":\\\\\\\"count\\\\\\\",\\\\\\\"value\\\\\\\":[\\\\\\\"D. Zeppenfeld\\\\\\\"],\\\\\\\"showMissing\\\\\\\":false,\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet\\\":\\\"true\\\",\\\"facet.field\\\":\\\"authors_ss\\\",\\\"facet.sort\\\":\\\"count\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"D. Zeppenfeld\\\",\\\"rows\\\":\\\"5\\\",\\\"sort\\\":\\\"id asc\\\",\\\"start\\\":\\\"0\\\"},\\\"result\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"result\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":[\\\\\\\"Authors_0\\\\\\\"]},\\\\\\\"dataField\\\\\\\":[\\\\\\\"_score\\\\\\\"],\\\\\\\"size\\\\\\\":9,\\\\\\\"sortBy\\\\\\\":\\\\\\\"desc\\\\\\\",\\\\\\\"highlight\\\\\\\":true,\\\\\\\"defaultQuery\\\\\\\":{\\\\\\\"track_total_hits\\\\\\\":true},\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"((((authors_ss:\\\\\\\"D. Zeppenfeld\\\\\\\"))))\\\",\\\"hl\\\":\\\"true\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"9\\\",\\\"sort\\\":\\\"score desc, id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"authors_ss\\\"],\\\"execute\\\":true,\\\"id\\\":\\\"Authors_0\\\",\\\"selectAllLabel\\\":null,\\\"showMissing\\\":false,\\\"size\\\":5,\\\"sortBy\\\":\\\"count\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":[\\\"D. Zeppenfeld\\\"]},{\\\"dataField\\\":[\\\"_score\\\"],\\\"defaultQuery\\\":{\\\"track_total_hits\\\":true},\\\"execute\\\":true,\\\"highlight\\\":true,\\\"id\\\":\\\"result\\\",\\\"react\\\":{\\\"and\\\":[\\\"Authors_0\\\"]},\\\"size\\\":9,\\\"sortBy\\\":\\\"desc\\\",\\\"type\\\":\\\"search\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: queryFormat properly applied for term if not passed", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":      "search",
					"type":    "search",
					"execute": false,
					"react": map[string]interface{}{
						"and": "search__internal",
					},
					"highlight": false,
					"size":      6,
					"value":     "vector",
				},
				map[string]interface{}{
					"id":   "Categories_inaccurate",
					"type": "term",
					"dataField": []interface{}{
						"categories_s",
					},
					"execute": true,
					"react": map[string]interface{}{
						"and": []interface{}{
							"search",
						},
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"Categories_inaccurate\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"Categories_inaccurate\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"term\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":[\\\\\\\"search\\\\\\\"]},\\\\\\\"dataField\\\\\\\":[\\\\\\\"categories_s\\\\\\\"],\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet\\\":\\\"true\\\",\\\"facet.field\\\":\\\"categories_s\\\",\\\"facet.sort\\\":\\\"count\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"vector*\\\",\\\"rows\\\":\\\"0\\\",\\\"sort\\\":\\\"id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"execute\\\":false,\\\"highlight\\\":false,\\\"id\\\":\\\"search\\\",\\\"react\\\":{\\\"and\\\":\\\"search__internal\\\"},\\\"size\\\":6,\\\"type\\\":\\\"search\\\",\\\"value\\\":\\\"vector\\\"},{\\\"dataField\\\":[\\\"categories_s\\\"],\\\"execute\\\":true,\\\"id\\\":\\\"Categories_inaccurate\\\",\\\"react\\\":{\\\"and\\\":[\\\"search\\\"]},\\\"type\\\":\\\"term\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: empty or * in react query property", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "result",
					"type": "search",
					"dataField": []interface{}{
						"title_t",
					},
					"execute": true,
					"react": map[string]interface{}{
						"and": []interface{}{
							"search",
						},
					},
					"size": 9,
				},
				map[string]interface{}{
					"id":   "search",
					"type": "search",
					"dataField": []interface{}{
						"title_t",
					},
					"execute": true,
					"react": map[string]interface{}{
						"and": "search__internal",
					},
					"value": "",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"result\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"result\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":[\\\\\\\"search\\\\\\\"]},\\\\\\\"dataField\\\\\\\":[\\\\\\\"title_t\\\\\\\"],\\\\\\\"size\\\\\\\":9,\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"(((title_t:*)))\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"9\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"},\\\"search\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"search\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"search__internal\\\\\\\"},\\\\\\\"dataField\\\\\\\":[\\\\\\\"title_t\\\\\\\"],\\\\\\\"value\\\\\\\":\\\\\\\"\\\\\\\",\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"title_t\\\"],\\\"execute\\\":true,\\\"id\\\":\\\"result\\\",\\\"react\\\":{\\\"and\\\":[\\\"search\\\"]},\\\"size\\\":9,\\\"type\\\":\\\"search\\\"},{\\\"dataField\\\":[\\\"title_t\\\"],\\\"execute\\\":true,\\\"id\\\":\\\"search\\\",\\\"react\\\":{\\\"and\\\":\\\"search__internal\\\"},\\\"type\\\":\\\"search\\\",\\\"value\\\":\\\"\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: _score is a special field", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id": "test",
					"dataField": []interface{}{
						"_score",
					},
					"sortBy": "desc",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"dataField\\\\\\\":[\\\\\\\"_score\\\\\\\"],\\\\\\\"sortBy\\\\\\\":\\\\\\\"desc\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc, id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"_score\\\"],\\\"id\\\":\\\"test\\\",\\\"sortBy\\\":\\\"desc\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: replace selectAllLabel with * if matches value", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "test",
					"type": "term",
					"dataField": []interface{}{
						"_score",
					},
					"value":          "tall",
					"selectAllLabel": "tall",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"term\\\\\\\",\\\\\\\"dataField\\\\\\\":[\\\\\\\"_score\\\\\\\"],\\\\\\\"value\\\\\\\":\\\\\\\"tall\\\\\\\",\\\\\\\"selectAllLabel\\\\\\\":\\\\\\\"tall\\\\\\\"}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet\\\":\\\"true\\\",\\\"facet.field\\\":\\\"\\\",\\\"facet.sort\\\":\\\"count\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"0\\\",\\\"sort\\\":\\\"id asc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"_score\\\"],\\\"id\\\":\\\"test\\\",\\\"selectAllLabel\\\":\\\"tall\\\",\\\"type\\\":\\\"term\\\",\\\"value\\\":\\\"tall\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})
}

func TestSolrSuggestionQuery(t *testing.T) {
	Convey("solr suggestion: highlight without field passed", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"value":     "quark",
					"dataField": "summary_t",
					"type":      "suggestion",
					"highlight": true,
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"suggestion\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\",\\\\\\\"highlight\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"true\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"quark*\\\",\\\"qf\\\":\\\"summary_t\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"highlight\\\":true,\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"suggestion\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr suggestion: highlight with field passed", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"value":     "quark",
					"dataField": "summary_t",
					"type":      "suggestion",
					"highlight": true,
					"highlightConfig": map[string]interface{}{
						"fields": map[string]interface{}{
							"summary_t": map[string]interface{}{},
						},
						"pre_tags": []interface{}{
							"<pre>",
						},
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"suggestion\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"summary_t\\\\\\\",\\\\\\\"value\\\\\\\":\\\\\\\"quark\\\\\\\",\\\\\\\"highlight\\\\\\\":true,\\\\\\\"highlightConfig\\\\\\\":{\\\\\\\"fields\\\\\\\":{\\\\\\\"summary_t\\\\\\\":{}},\\\\\\\"pre_tags\\\\\\\":[\\\\\\\"\\\\\\\\u003cpre\\\\\\\\u003e\\\\\\\"]}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"true\\\",\\\"hl.fl\\\":\\\"summary_t\\\",\\\"hl.simple.pre\\\":\\\"\\\\u003cpre\\\\u003e\\\",\\\"q\\\":\\\"quark*\\\",\\\"qf\\\":\\\"summary_t\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"summary_t\\\",\\\"highlight\\\":true,\\\"highlightConfig\\\":{\\\"fields\\\":{\\\"summary_t\\\":{}},\\\"pre_tags\\\":[\\\"\\\\u003cpre\\\\u003e\\\"]},\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"suggestion\\\",\\\"value\\\":\\\"quark\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"
		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})
}

func TestSolrGeoQuery(t *testing.T) {
	Convey("solr geo: km as unit", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"dataField": "location_t",
					"type":      "geo",
					"value": map[string]interface{}{
						"location": "32.7906865,-96.7979007",
						"distance": 99099,
						"unit":     "km",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"geo\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"location_t\\\\\\\",\\\\\\\"value\\\\\\\":{\\\\\\\"distance\\\\\\\":99099,\\\\\\\"location\\\\\\\":\\\\\\\"32.7906865,-96.7979007\\\\\\\",\\\\\\\"unit\\\\\\\":\\\\\\\"km\\\\\\\"}}\\\",\\\"d\\\":\\\"99099.000000\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"{!geofilt}\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"pt\\\":\\\"32.7906865,-96.7979007\\\",\\\"q\\\":\\\"*\\\",\\\"qf\\\":\\\"location_t\\\",\\\"rows\\\":\\\"10\\\",\\\"sfield\\\":\\\"location_t\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"location_t\\\",\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"geo\\\",\\\"value\\\":{\\\"distance\\\":99099,\\\"location\\\":\\\"32.7906865,-96.7979007\\\",\\\"unit\\\":\\\"km\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr geo: non km as unit", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"dataField": "location_t",
					"type":      "geo",
					"value": map[string]interface{}{
						"location": "32.7906865,-96.7979007",
						"distance": 6000,
						"unit":     "mi",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"geo\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"location_t\\\\\\\",\\\\\\\"value\\\\\\\":{\\\\\\\"distance\\\\\\\":6000,\\\\\\\"location\\\\\\\":\\\\\\\"32.7906865,-96.7979007\\\\\\\",\\\\\\\"unit\\\\\\\":\\\\\\\"mi\\\\\\\"}}\\\",\\\"d\\\":\\\"9656.064000\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"{!geofilt}\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"pt\\\":\\\"32.7906865,-96.7979007\\\",\\\"q\\\":\\\"*\\\",\\\"qf\\\":\\\"location_t\\\",\\\"rows\\\":\\\"10\\\",\\\"sfield\\\":\\\"location_t\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"location_t\\\",\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"geo\\\",\\\"value\\\":{\\\"distance\\\":6000,\\\"location\\\":\\\"32.7906865,-96.7979007\\\",\\\"unit\\\":\\\"mi\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr geo: bounding box", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"dataField": "location_t",
					"type":      "geo",
					"value": map[string]interface{}{
						"geoBoundingBox": map[string]interface{}{
							"topLeft":     "32.7906865,-96.7979007",
							"bottomRight": "33.7906866,-95.7979006",
						},
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"geo\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"location_t\\\\\\\",\\\\\\\"value\\\\\\\":{\\\\\\\"geoBoundingBox\\\\\\\":{\\\\\\\"bottomRight\\\\\\\":\\\\\\\"33.7906866,-95.7979006\\\\\\\",\\\\\\\"topLeft\\\\\\\":\\\\\\\"32.7906865,-96.7979007\\\\\\\"}}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"location_t:[33.790687,-96.797901 TO 32.790686,-95.797901]\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"qf\\\":\\\"location_t\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"location_t\\\",\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"geo\\\",\\\"value\\\":{\\\"geoBoundingBox\\\":{\\\"bottomRight\\\":\\\"33.7906866,-95.7979006\\\",\\\"topLeft\\\":\\\"32.7906865,-96.7979007\\\"}}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr geo: bounding box reacted upon", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":        "test",
					"dataField": "location_t",
					"type":      "geo",
					"value": map[string]interface{}{
						"geoBoundingBox": map[string]interface{}{
							"topLeft":     "32.7906865,-96.7979007",
							"bottomRight": "33.7906866,-95.7979006",
						},
					},
					"execute": false,
				},
				map[string]interface{}{
					"id": "second",
					"react": map[string]interface{}{
						"and": "test",
					}},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"second\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"second\\\\\\\",\\\\\\\"react\\\\\\\":{\\\\\\\"and\\\\\\\":\\\\\\\"test\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"fq\\\":\\\"(location_t:\\\\\\\"[33.790687,-96.797901 TO 32.790686,-95.797901]\\\\\\\")\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":\\\"location_t\\\",\\\"execute\\\":false,\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"geo\\\",\\\"value\\\":{\\\"geoBoundingBox\\\":{\\\"bottomRight\\\":\\\"33.7906866,-95.7979006\\\",\\\"topLeft\\\":\\\"32.7906865,-96.7979007\\\"}}},{\\\"id\\\":\\\"second\\\",\\\"react\\\":{\\\"and\\\":\\\"test\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})
}

func TestSolrRangeQuery(t *testing.T) {
	Convey("solr range: basic", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "test",
					"type": "range",
					"value": map[string]interface{}{
						"start": "1996-01-01",
						"end":   "2022-01-01",
					},
					"aggregations": []interface{}{
						"histogram",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"range\\\\\\\",\\\\\\\"value\\\\\\\":{\\\\\\\"end\\\\\\\":\\\\\\\"2022-01-01\\\\\\\",\\\\\\\"start\\\\\\\":\\\\\\\"1996-01-01\\\\\\\"},\\\\\\\"aggregations\\\\\\\":[\\\\\\\"histogram\\\\\\\"]}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet.range\\\":\\\"\\\",\\\"facet.range.end\\\":\\\"2022-01-01T00:00:00Z\\\",\\\"facet.range.gap\\\":\\\"+1MONTH\\\",\\\"facet.range.start\\\":\\\"1996-01-01T00:00:00Z\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"aggregations\\\":[\\\"histogram\\\"],\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"range\\\",\\\"value\\\":{\\\"end\\\":\\\"2022-01-01\\\",\\\"start\\\":\\\"1996-01-01\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr range: time with tz", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "test",
					"type": "range",
					"value": map[string]interface{}{
						"start": "1996-01-01T05:00:00Z",
						"end":   "2022-01-01T05:00:00Z",
					},
					"aggregations": []interface{}{
						"histogram",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"range\\\\\\\",\\\\\\\"value\\\\\\\":{\\\\\\\"end\\\\\\\":\\\\\\\"2022-01-01T05:00:00Z\\\\\\\",\\\\\\\"start\\\\\\\":\\\\\\\"1996-01-01T05:00:00Z\\\\\\\"},\\\\\\\"aggregations\\\\\\\":[\\\\\\\"histogram\\\\\\\"]}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet.range\\\":\\\"\\\",\\\"facet.range.end\\\":\\\"2022-01-01T05:00:00Z\\\",\\\"facet.range.gap\\\":\\\"+1MONTH\\\",\\\"facet.range.start\\\":\\\"1996-01-01T05:00:00Z\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"aggregations\\\":[\\\"histogram\\\"],\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"range\\\",\\\"value\\\":{\\\"end\\\":\\\"2022-01-01T05:00:00Z\\\",\\\"start\\\":\\\"1996-01-01T05:00:00Z\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr range: histogram with min and max", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "test",
					"type": "range",
					"value": map[string]interface{}{
						"start": "1996-01-01T05:00:00Z",
						"end":   "2022-01-01T05:00:00Z",
					},
					"aggregations": []interface{}{
						"histogram",
						"min",
						"max",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"range\\\\\\\",\\\\\\\"value\\\\\\\":{\\\\\\\"end\\\\\\\":\\\\\\\"2022-01-01T05:00:00Z\\\\\\\",\\\\\\\"start\\\\\\\":\\\\\\\"1996-01-01T05:00:00Z\\\\\\\"},\\\\\\\"aggregations\\\\\\\":[\\\\\\\"histogram\\\\\\\",\\\\\\\"min\\\\\\\",\\\\\\\"max\\\\\\\"]}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet.range\\\":\\\"\\\",\\\"facet.range.end\\\":\\\"2022-01-01T05:00:00Z\\\",\\\"facet.range.gap\\\":\\\"+1MONTH\\\",\\\"facet.range.start\\\":\\\"1996-01-01T05:00:00Z\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\",\\\"stats\\\":\\\"true\\\",\\\"stats.field\\\":\\\"\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"aggregations\\\":[\\\"histogram\\\",\\\"min\\\",\\\"max\\\"],\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"range\\\",\\\"value\\\":{\\\"end\\\":\\\"2022-01-01T05:00:00Z\\\",\\\"start\\\":\\\"1996-01-01T05:00:00Z\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr range: with dataField specified", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "test",
					"type": "range",
					"value": map[string]interface{}{
						"start": "1996-01-01T05:00:00Z",
						"end":   "2022-01-01T05:00:00Z",
					},
					"aggregations": []interface{}{
						"histogram",
					},
					"dataField": "timestamp_dt",
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"range\\\\\\\",\\\\\\\"dataField\\\\\\\":\\\\\\\"timestamp_dt\\\\\\\",\\\\\\\"value\\\\\\\":{\\\\\\\"end\\\\\\\":\\\\\\\"2022-01-01T05:00:00Z\\\\\\\",\\\\\\\"start\\\\\\\":\\\\\\\"1996-01-01T05:00:00Z\\\\\\\"},\\\\\\\"aggregations\\\\\\\":[\\\\\\\"histogram\\\\\\\"]}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"facet.range\\\":\\\"timestamp_dt\\\",\\\"facet.range.end\\\":\\\"2022-01-01T05:00:00Z\\\",\\\"facet.range.gap\\\":\\\"+1MONTH\\\",\\\"facet.range.start\\\":\\\"1996-01-01T05:00:00Z\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"aggregations\\\":[\\\"histogram\\\"],\\\"dataField\\\":\\\"timestamp_dt\\\",\\\"id\\\":\\\"test\\\",\\\"type\\\":\\\"range\\\",\\\"value\\\":{\\\"end\\\":\\\"2022-01-01T05:00:00Z\\\",\\\"start\\\":\\\"1996-01-01T05:00:00Z\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: range query with value", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "range-test",
					"type": "range",
					"dataField": []interface{}{
						"_lw_parser_line_number_l",
					},
					"execute": true,
					"size":    10,
					"value": map[string]interface{}{
						"start": 0,
						"end":   10000,
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"range-test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"range-test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"range\\\\\\\",\\\\\\\"dataField\\\\\\\":[\\\\\\\"_lw_parser_line_number_l\\\\\\\"],\\\\\\\"size\\\\\\\":10,\\\\\\\"value\\\\\\\":{\\\\\\\"end\\\\\\\":10000,\\\\\\\"start\\\\\\\":0},\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"[0 TO 10000]\\\",\\\"qf\\\":\\\"_lw_parser_line_number_l\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"_lw_parser_line_number_l\\\"],\\\"execute\\\":true,\\\"id\\\":\\\"range-test\\\",\\\"size\\\":10,\\\"type\\\":\\\"range\\\",\\\"value\\\":{\\\"end\\\":10000,\\\"start\\\":0}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: range query without value", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "range-test",
					"type": "range",
					"dataField": []interface{}{
						"_lw_parser_line_number_l",
					},
					"execute": true,
					"size":    10,
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"range-test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"range-test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"range\\\\\\\",\\\\\\\"dataField\\\\\\\":[\\\\\\\"_lw_parser_line_number_l\\\\\\\"],\\\\\\\"size\\\\\\\":10,\\\\\\\"execute\\\\\\\":true}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"[__min__ TO __max__]\\\",\\\"qf\\\":\\\"_lw_parser_line_number_l\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"_lw_parser_line_number_l\\\"],\\\"execute\\\":true,\\\"id\\\":\\\"range-test\\\",\\\"size\\\":10,\\\"type\\\":\\\"range\\\"}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})

	Convey("solr search: append timezone to value if not passed", t, func() {
		requestBody := map[string]interface{}{
			"query": []interface{}{
				map[string]interface{}{
					"id":   "test",
					"type": "range",
					"dataField": []interface{}{
						"release_date",
					},
					"size": 10,
					"value": map[string]interface{}{
						"start": "2018-01-01",
						"end":   "2022-01-01",
					},
				},
			},
		}

		updatedContextInBytes, _, err := runTestStep(requestBody)

		So(err, ShouldBeNil)

		expectedContext := "{\"request\":{\"body\":\"{\\\"test\\\":{\\\"\\\":\\\"\\\",\\\"_index\\\":\\\"\\\",\\\"_original\\\":\\\"{\\\\\\\"id\\\\\\\":\\\\\\\"test\\\\\\\",\\\\\\\"type\\\\\\\":\\\\\\\"range\\\\\\\",\\\\\\\"dataField\\\\\\\":[\\\\\\\"release_date\\\\\\\"],\\\\\\\"size\\\\\\\":10,\\\\\\\"value\\\\\\\":{\\\\\\\"end\\\\\\\":\\\\\\\"2022-01-01\\\\\\\",\\\\\\\"start\\\\\\\":\\\\\\\"2018-01-01\\\\\\\"}}\\\",\\\"defType\\\":\\\"edismax\\\",\\\"fl\\\":\\\"score,*\\\",\\\"hl\\\":\\\"false\\\",\\\"hl.fl\\\":\\\"*\\\",\\\"q\\\":\\\"[2018-01-01T00:00:00Z TO 2022-01-01T00:00:00Z]\\\",\\\"qf\\\":\\\"release_date\\\",\\\"rows\\\":\\\"10\\\",\\\"sort\\\":\\\"score desc\\\",\\\"start\\\":\\\"0\\\"}}\",\"headers\":null,\"url\":\"\",\"method\":\"\"},\"response\":{\"code\":200,\"body\":\"\",\"headers\":{}},\"envs\":{\"INDEPENDENT_REQUESTS\":\"[]\",\"ML_QS_PARAMS\":\"\",\"ORIGINAL_RS_BODY\":\"{\\\"query\\\":[{\\\"dataField\\\":[\\\"release_date\\\"],\\\"id\\\":\\\"test\\\",\\\"size\\\":10,\\\"type\\\":\\\"range\\\",\\\"value\\\":{\\\"end\\\":\\\"2022-01-01\\\",\\\"start\\\":\\\"2018-01-01\\\"}}]}\",\"RS_BACKEND\":\"solr\"}}"

		So(string(updatedContextInBytes), ShouldResemble, expectedContext)
	})
}
