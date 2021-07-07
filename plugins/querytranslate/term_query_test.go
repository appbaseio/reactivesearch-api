package querytranslate

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

// Failed
func TestMultiListWithDefaultValue(t *testing.T) {
	convey.Convey("with default value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      100,
					"dataField": []string{"original_series.raw"},
					"value":     []string{"San Francisco"},
					"type":      "term",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"original_series.raw":{"composite":{"size":100,"sources":[{"original_series.raw":{"terms":{"field":"original_series.raw"}}}]}}},"query":{"bool":{"should":[{"terms":{"original_series.raw":["San Francisco"]}}]}},"size":0}
`)
	})
}

// Failed
func TestMultiDropdownList(t *testing.T) {
	convey.Convey("with value and query format", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":          "BookSensor",
					"dataField":   []string{"original_series.raw"},
					"value":       []string{"In Death", "Discworld"},
					"type":        "term",
					"queryFormat": "and",
				},
				{
					"id":        "SearchResult",
					"size":      10,
					"dataField": []string{"original_title.raw"},
					"react": map[string]interface{}{
						"and": "BookSensor",
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"original_series.raw":{"composite":{"sources":[{"original_series.raw":{"terms":{"field":"original_series.raw"}}}]}}},"query":{"bool":{"must":[{"term":{"original_series.raw":"In Death"}},{"term":{"original_series.raw":"Discworld"}}]}},"size":0}
{"preference":"SearchResult"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":{"bool":{"must":[{"term":{"original_series.raw":"In Death"}},{"term":{"original_series.raw":"Discworld"}}]}}}}]}},"size":10}
`)
	})
}

// Failed
func TestMultiDataList(t *testing.T) {
	convey.Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "CitySensor",
					"dataField": []string{"group.group_topics.topic_name_raw.raw"},
					"value":     []string{"Social", "Adventure"},
					"type":      "term",
				},
				{
					"id":        "SearchResult",
					"size":      5,
					"dataField": []string{"group.group_topics.topic_name_raw"},
					"react": map[string]interface{}{
						"and": []string{"CitySensor"},
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"CitySensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"group.group_topics.topic_name_raw.raw":{"composite":{"sources":[{"group.group_topics.topic_name_raw.raw":{"terms":{"field":"group.group_topics.topic_name_raw.raw"}}}]}}},"query":{"bool":{"should":[{"terms":{"group.group_topics.topic_name_raw.raw":["Social","Adventure"]}}]}},"size":0}
{"preference":"SearchResult"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":[{"bool":{"should":[{"terms":{"group.group_topics.topic_name_raw.raw":["Social","Adventure"]}}]}}]}}]}},"size":5}
`)
	})
}

// Failed
func TestSingleDataList(t *testing.T) {
	convey.Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "CitySensor",
					"dataField": []string{"group.group_topics.topic_name_raw.raw"},
					"value":     "Social",
					"type":      "term",
				},
				{
					"id":        "SearchResult",
					"size":      5,
					"dataField": []string{"group.group_topics.topic_name_raw"},
					"react": map[string]interface{}{
						"and": []string{"CitySensor"},
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"CitySensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"group.group_topics.topic_name_raw.raw":{"composite":{"sources":[{"group.group_topics.topic_name_raw.raw":{"terms":{"field":"group.group_topics.topic_name_raw.raw"}}}]}}},"query":{"term":{"group.group_topics.topic_name_raw.raw":"Social"}},"size":0}
{"preference":"SearchResult"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":[{"term":{"group.group_topics.topic_name_raw.raw":"Social"}}]}}]}},"size":5}
`)
	})
}

// Failed
func TestToggle(t *testing.T) {
	convey.Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "CitySensor",
					"dataField": []string{"group.group_topics.topic_name_raw.raw"},
					"value":     []string{"Social", "Adventure"},
					"type":      "term",
				},
				{
					"id":        "SearchResult",
					"size":      5,
					"dataField": []string{"group.group_topics.topic_name_raw"},
					"react": map[string]interface{}{
						"and": []string{"CitySensor"},
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"CitySensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"group.group_topics.topic_name_raw.raw":{"composite":{"sources":[{"group.group_topics.topic_name_raw.raw":{"terms":{"field":"group.group_topics.topic_name_raw.raw"}}}]}}},"query":{"bool":{"should":[{"terms":{"group.group_topics.topic_name_raw.raw":["Social","Adventure"]}}]}},"size":0}
{"preference":"SearchResult"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":[{"bool":{"should":[{"terms":{"group.group_topics.topic_name_raw.raw":["Social","Adventure"]}}]}}]}}]}},"size":5}
`)
	})
}

// Failed
func TestMultiListWithQueryFormat(t *testing.T) {
	convey.Convey("with query format", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":          "BookSensor",
					"size":        100,
					"dataField":   []string{"original_series.raw"},
					"type":        "term",
					"queryFormat": "and",
					"value":       []string{"San Fransisco"},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"original_series.raw":{"composite":{"size":100,"sources":[{"original_series.raw":{"terms":{"field":"original_series.raw"}}}]}}},"query":{"bool":{"must":[{"term":{"original_series.raw":"San Fransisco"}}]}},"size":0}
`)
	})
}

// Failed
func TestMultiListWithMissingBucket(t *testing.T) {
	convey.Convey("with query format", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":          "BookSensor",
					"size":        100,
					"dataField":   []string{"original_series.raw"},
					"type":        "term",
					"queryFormat": "and",
					"value":       []string{"San Fransisco"},
					"showMissing": true,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"original_series.raw":{"composite":{"size":100,"sources":[{"original_series.raw":{"terms":{"field":"original_series.raw","missing_bucket":true}}}]}}},"query":{"bool":{"must":[{"term":{"original_series.raw":"San Fransisco"}}]}},"size":0}
`)
	})
}

// Failed
func TestMultiListWithAfterKey(t *testing.T) {
	convey.Convey("with after key", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      10,
					"dataField": []string{"brand.keyword"},
					"type":      "term",
					"after": map[string]interface{}{
						"brand.keyword": "Chevrolet",
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"brand.keyword":{"composite":{"after":{"brand.keyword":"Chevrolet"},"size":10,"sources":[{"brand.keyword":{"terms":{"field":"brand.keyword"}}}]}}},"query":{"match_all":{}},"size":0}
`)
	})
}

// Failed
func TestMultiListWithDefaultQuery(t *testing.T) {
	convey.Convey("with default query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "SearchResult",
					"dataField": []string{"original_series.raw"},
					"defaultQuery": map[string]interface{}{
						"query": map[string]interface{}{
							"terms": map[string]interface{}{
								"country": []string{"India"},
							},
						},
					},
					"type":  "term",
					"value": []string{"San Fransisco"},
					"size":  100,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"SearchResult"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"original_series.raw":{"composite":{"size":100,"sources":[{"original_series.raw":{"terms":{"field":"original_series.raw"}}}]}}},"query":{"terms":{"country":["India"]}},"size":0}
`)
	})
}

// Failed
func TestMultiListWithCustomQuery(t *testing.T) {
	convey.Convey("with custom query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_series.raw"},
					"customQuery": map[string]interface{}{
						"query": map[string]interface{}{
							"term": map[string]interface{}{
								"city": "San Fransisco",
							},
						},
					},
					"type":  "term",
					"value": []string{"San Fransisco"},
					"size":  100,
				},
				{
					"id":        "SearchResult",
					"dataField": []string{"original_series"},
					"size":      100,
					"react": map[string]interface{}{
						"and": "BookSensor",
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"original_series.raw":{"composite":{"size":100,"sources":[{"original_series.raw":{"terms":{"field":"original_series.raw"}}}]}}},"query":{"bool":{"should":[{"terms":{"original_series.raw":["San Fransisco"]}}]}},"size":0}
{"preference":"SearchResult"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":{"term":{"city":"San Fransisco"}}}}]}},"size":100}
`)
	})
}

// Failed
func TestMultiListWithSortAsc(t *testing.T) {
	convey.Convey("with sort ascending", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      10,
					"dataField": []string{"brand.keyword"},
					"type":      "term",
					"after": map[string]interface{}{
						"brand.keyword": "Maybach",
					},
					"sortBy": "desc",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"brand.keyword":{"composite":{"after":{"brand.keyword":"Maybach"},"size":10,"sources":[{"brand.keyword":{"terms":{"field":"brand.keyword","order":"desc"}}}]}}},"query":{"match_all":{}},"size":0}
`)
	})
}

// Failed
func TestMultiListWithSortByCount(t *testing.T) {
	convey.Convey("with sortBy count", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      10,
					"dataField": []string{"brand.keyword"},
					"type":      "term",
					"after": map[string]interface{}{
						"brand.keyword": "Chevrolet",
					},
					"sortBy": "count",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"brand.keyword":{"composite":{"after":{"brand.keyword":"Chevrolet"},"size":10,"sources":[{"brand.keyword":{"terms":{"field":"brand.keyword"}}}]}}},"query":{"match_all":{}},"size":0}
`)
	})
}
