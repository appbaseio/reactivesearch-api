package querytranslate

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func transformQuery(query map[string]interface{}) (string, error) {
	var body RSQuery
	marshalled, err := json.Marshal(query)
	if err != nil {
		return "", err
	}
	err2 := json.Unmarshal(marshalled, &body)
	if err2 != nil {
		return "", err2
	}
	return translateQuery(body, "127.0.0.1")
}

func TestQueryWithValue(t *testing.T) {
	Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_title"},
					"value":     "harry",
					"size":      20,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}},"size":20}
`)
	})
}

func TestBasicQuery(t *testing.T) {
	Convey("basic query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_title"},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"match_all":{}}}
`)
	})
}

func TestWithMultipleDataFields(t *testing.T) {
	Convey("with multiple data fields", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_title", "original_title.raw"},
					"value":     "harry",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title","original_title.raw"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title","original_title.raw"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title","original_title.raw"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title","original_title.raw"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}}}
`)
	})
}

func TestSearchWithNoDataField(t *testing.T) {
	Convey("should not throw error when value is not defined and `dataField` is not defined and `react` property is not defined and `defaultQuery` is not defined", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id": "BookSensor",
				},
			},
		}
		_, err := transformQuery(query)
		So(err, ShouldBeNil)
	})
	Convey("should throw error when value is defined and `dataField` is not defined and `react` property is not defined and `defaultQuery` is present with no `query` key", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":    "BookSensor",
					"value": "harry",
					"defaultQuery": map[string]interface{}{
						"size": 10,
					},
				},
			},
		}
		_, err := transformQuery(query)
		So(err, ShouldNotBeNil)
	})
	Convey("should throw error when value is defined, dataField, defaultQuery aren't defined, react prop is defined but has a nil query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":    "Results",
					"value": "harry",
					"react": map[string]interface{}{
						"and": "BookSensor",
					},
				},
			},
		}
		_, err := transformQuery(query)
		So(err, ShouldNotBeNil)
	})
	Convey("should not throw error when dataField is not defined and `defaultQuery` has `query` key", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":    "BookSensor",
					"value": "harry",
					"defaultQuery": map[string]interface{}{
						"size": 10,
						"query": map[string]interface{}{
							"match": map[string]interface{}{
								"title": "harry",
							},
						},
					},
				},
			},
		}
		_, err := transformQuery(query)
		So(err, ShouldBeNil)
	})
	Convey("should throw when `sortBy` is defined but dataField isn't defined, even if a defaultQuery is present", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":     "BookSensor",
					"value":  "harry",
					"sortBy": "asc",
					"defaultQuery": map[string]interface{}{
						"size": 10,
						"query": map[string]interface{}{
							"match": map[string]interface{}{
								"title": "harry",
							},
						},
					},
				},
			},
		}
		_, err := transformQuery(query)
		So(err, ShouldNotBeNil)
	})
}

func TestWithFieldWeights(t *testing.T) {
	Convey("with multiple field weights", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":           "BookSensor",
					"dataField":    []string{"original_title", "original_title.raw"},
					"fieldWeights": []float64{1, 3},
					"value":        "harry",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title^1.00","original_title.raw^3.00"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title^1.00","original_title.raw^3.00"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title^1.00","original_title.raw^3.00"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title^1.0","original_title.raw^1.0"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}}}
`)
	})
}

func TestWithFieldWeightsNewFromat(t *testing.T) {
	Convey("with multiple fields with weights", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id": "BookSensor",
					"dataField": []interface{}{
						map[string]interface{}{
							"field":  "original_title",
							"weight": 1,
						},
						map[string]interface{}{
							"field":  "original_title.raw",
							"weight": 3,
						},
					},
					"value": "harry",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title^1.00","original_title.raw^3.00"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title^1.00","original_title.raw^3.00"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title^1.00","original_title.raw^3.00"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title^1.0","original_title.raw^1.0"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}}}
`)
	})
}

func TestQueryFormat(t *testing.T) {
	Convey("with query format", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":          "BookSensor",
					"dataField":   []string{"original_title"},
					"queryFormat": "and",
					"value":       "harry",
					"size":        20,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"and","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"operator":"and","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"and","query":"harry","type":"phrase_prefix"}}]}},"size":20}
`)
	})
}

func TestSearchOperators(t *testing.T) {
	Convey("with search operators", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":              "BookSensor",
					"dataField":       []string{"original_title"},
					"searchOperators": true,
					"value":           "^harry",
					"size":            20,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"simple_query_string":{"default_operator":"or","fields":["original_title"],"query":"^harry"}},"size":20}
`)
	})
}

func TestQueryWithFuzziness(t *testing.T) {
	Convey("with fuzziness", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_title"},
					"value":     "harry",
					"size":      20,
					"fuzziness": 2,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":2,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}},"size":20}
`)
	})
}

func TestQueryWithIncludeFields(t *testing.T) {
	Convey("with include fields", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":            "BookSensor",
					"dataField":     []string{"original_title"},
					"value":         "Harry Potter Collection (Harry Potter, #1-6)",
					"size":          10,
					"includeFields": []string{"original_title.raw"},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["original_title.raw"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"phrase_prefix"}}]}},"size":10}
`)
	})
}

func TestQueryWithExcludeFields(t *testing.T) {
	Convey("with exclude fields", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":            "BookSensor",
					"dataField":     []string{"original_title"},
					"value":         "Harry Potter Collection (Harry Potter, #1-6)",
					"size":          10,
					"excludeFields": []string{"original_title.raw"},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":["original_title.raw"],"includes":["*"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harry Potter Collection (Harry Potter, #1-6)","type":"phrase_prefix"}}]}},"size":10}
`)
	})
}

func TestQueryFrom(t *testing.T) {
	Convey("with `from`", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_title"},
					"value":     "Harvesting the Heart",
					"size":      20,
					"from":      40,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"from":40,"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harvesting the Heart","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"Harvesting the Heart","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harvesting the Heart","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"Harvesting the Heart","type":"phrase_prefix"}}]}},"size":20}
`)
	})
}

func TestQueryHighlight(t *testing.T) {
	Convey("with highlight", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_title"},
					"value":     "harry",
					"size":      20,
					"highlight": true,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"highlight":{"fields":{"original_title":{}},"post_tags":["\u003c/mark\u003e"],"pre_tags":["\u003cmark\u003e"]},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}},"size":20}
`)
	})
}

func TestQueryHighlightField(t *testing.T) {
	Convey("with highlight field", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":             "BookSensor",
					"dataField":      []string{"original_title"},
					"value":          "harry",
					"size":           20,
					"highlight":      true,
					"highlightField": []string{"original_title"},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"highlight":{"fields":{"original_title":{}},"post_tags":["\u003c/mark\u003e"],"pre_tags":["\u003cmark\u003e"],"require_field_match":false},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}},"size":20}
`)
	})
}

func TestQueryNestedField(t *testing.T) {
	Convey("with nested field", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":          "BookSensor",
					"dataField":   []string{"original_title"},
					"value":       "harry",
					"size":        20,
					"nestedField": "original_title.raw",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"nested":{"path":"original_title.raw","query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}}}},"size":20}
`)
	})
}

func TestQueryWithAggregationField(t *testing.T) {
	Convey("with aggregation field", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":               "CarSensor",
					"dataField":        []string{"brand"},
					"value":            "bmw",
					"size":             10,
					"aggregationField": "brand.keyword",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"CarSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"brand.keyword":{"aggs":{"brand.keyword":{"top_hits":{"size":1}}},"composite":{"size":10,"sources":[{"brand.keyword":{"terms":{"field":"brand.keyword"}}}]}}},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["brand"],"operator":"or","query":"bmw","type":"cross_fields"}},{"multi_match":{"fields":["brand"],"fuzziness":0,"operator":"or","query":"bmw","type":"best_fields"}},{"multi_match":{"fields":["brand"],"operator":"or","query":"bmw","type":"phrase"}},{"multi_match":{"fields":["brand"],"operator":"or","query":"bmw","type":"phrase_prefix"}}]}},"size":10}
`)
	})
}

func TestQueryWithCategoryField(t *testing.T) {
	Convey("with category field", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":            "BookSensor",
					"dataField":     []string{"original_title", "original_title.search"},
					"value":         "harry",
					"size":          10,
					"categoryField": "authors.raw",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"authors.raw":{"terms":{"field":"authors.raw"}}},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title","original_title.search"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title","original_title.search"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title","original_title.search"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}},"size":10}
`)
	})
}

func TestQueryWithCategories(t *testing.T) {
	Convey("with category keyword", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":            "BookSensor",
					"dataField":     []string{"original_title", "original_title.search"},
					"value":         "harry",
					"size":          10,
					"categoryField": "authors.raw",
					"categoryValue": "J.K. Rowling",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"authors.raw":{"terms":{"field":"authors.raw"}}},"query":{"bool":{"must":[{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title","original_title.search"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title","original_title.search"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title","original_title.search"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}},{"term":{"authors.raw":"J.K. Rowling"}}]}},"size":10}
`)
	})
}

func TestQueryWithCategoryAndAggregationField(t *testing.T) {
	Convey("with category and aggregation field", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":               "BookSensor",
					"dataField":        []string{"original_title", "original_title.search"},
					"value":            "ha",
					"size":             10,
					"categoryField":    "title.keyword",
					"aggregationField": "original_title.keyword",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"original_title.keyword":{"aggs":{"original_title.keyword":{"top_hits":{"size":1}}},"composite":{"size":10,"sources":[{"original_title.keyword":{"terms":{"field":"original_title.keyword"}}}]}},"title.keyword":{"terms":{"field":"title.keyword"}}},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title","original_title.search"],"operator":"or","query":"ha","type":"cross_fields"}},{"multi_match":{"fields":["original_title","original_title.search"],"fuzziness":0,"operator":"or","query":"ha","type":"best_fields"}},{"multi_match":{"fields":["original_title","original_title.search"],"operator":"or","query":"ha","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"ha","type":"phrase_prefix"}}]}},"size":10}
`)
	})
}

// Failed
func TestCategorySearchWithQueryFormat(t *testing.T) {
	Convey("with query format", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":            "BookSensor",
					"dataField":     []string{"original_title", "original_title.search"},
					"value":         "harry",
					"size":          10,
					"categoryField": "authors.raw",
					"queryFormat":   "and",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"BookSensor_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"authors.raw":{"terms":{"field":"authors.raw"}}},"query":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title","original_title.search"],"operator":"and","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title","original_title.search"],"operator":"and","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"and","query":"harry","type":"phrase_prefix"}}]}},"size":10}
`)
	})
}

func TestBasicReactiveList(t *testing.T) {
	Convey("basic", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "SearchResult",
					"size":      3,
					"dataField": []string{"original_title", "original_title.search"},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"match_all":{}},"size":3}
`)
	})
}

func TestBasicReactiveListWithSortAscending(t *testing.T) {
	Convey("with sort ascending", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "SearchResult",
					"size":      3,
					"dataField": []string{"original_title.raw"},
					"sortBy":    "asc",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"match_all":{}},"size":3,"sort":[{"original_title.raw":{"order":"asc"}}]}
`)
	})
}

func TestBasicReactiveListWithSortDescending(t *testing.T) {
	Convey("with sort descending", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "SearchResult",
					"size":      3,
					"dataField": []string{"original_title.raw"},
					"sortBy":    "desc",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"match_all":{}},"size":3,"sort":[{"original_title.raw":{"order":"desc"}}]}
`)
	})
}

func TestReactiveAnd(t *testing.T) {
	Convey("with react and", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      20,
					"dataField": []string{"original_title"},
					"value":     "harry",
					"execute":   false,
				},
				{
					"id":        "SearchResult",
					"size":      20,
					"dataField": []string{"original_title"},
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
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}}}}]}},"size":20}
`)
	})
}

func TestReactiveOr(t *testing.T) {
	Convey("with react or", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      20,
					"dataField": []string{"original_title"},
					"value":     "harry",
					"execute":   false,
				},
				{
					"id":        "SearchResult",
					"size":      20,
					"dataField": []string{"original_title"},
					"react": map[string]interface{}{
						"or": "BookSensor",
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"minimum_should_match":1,"should":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}}}}]}},"size":20}
`)
	})
}

func TestReactiveNot(t *testing.T) {
	Convey("with react not", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      20,
					"dataField": []string{"original_title"},
					"value":     "harry",
					"execute":   false,
				},
				{
					"id":        "SearchResult",
					"size":      20,
					"dataField": []string{"original_title"},
					"react": map[string]interface{}{
						"not": "BookSensor",
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must_not":{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}}}}]}},"size":20}
`)
	})
}

func TestReactiveWithArray(t *testing.T) {
	Convey("with react array", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      20,
					"dataField": []string{"original_title"},
					"value":     "harry",
					"execute":   false,
				},
				{
					"id":        "BookSensor2",
					"size":      20,
					"dataField": []string{"original_title"},
					"value":     "potter",
					"execute":   false,
				},
				{
					"id":        "SearchResult",
					"size":      20,
					"dataField": []string{"original_title"},
					"react": map[string]interface{}{
						"or": []string{"BookSensor", "BookSensor2"},
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"minimum_should_match":1,"should":[{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"harry","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"harry","type":"phrase_prefix"}}]}},{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"potter","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"potter","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"potter","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"potter","type":"phrase_prefix"}}]}}]}}]}},"size":20}
`)
	})
}

func TestBasicReactiveListWithDefaultQuery(t *testing.T) {
	Convey("with default query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "SearchResult",
					"size":      3,
					"dataField": []string{"original_title.raw"},
					"defaultQuery": map[string]interface{}{
						"query": map[string]interface{}{
							"terms": map[string]interface{}{
								"country": []string{"India"},
							},
						},
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"terms":{"country":["India"]}},"size":3}
`)
	})
}

func TestBasicReactiveListWithDefaultQueryWithoutField(t *testing.T) {
	Convey("without dataField and with default query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":   "SearchResult",
					"size": 3,
					"defaultQuery": map[string]interface{}{
						"query": map[string]interface{}{
							"terms": map[string]interface{}{
								"country": []string{"India"},
							},
						},
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"terms":{"country":["India"]}},"size":3}
`)
	})
}
func TestBasicDataSearchWithDefaultQuery(t *testing.T) {
	Convey("with default query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "SearchResult",
					"size":      3,
					"dataField": []string{"original_title.raw"},
					"sortBy":    "desc",
					"defaultQuery": map[string]interface{}{
						"query": map[string]interface{}{
							"terms": map[string]interface{}{
								"country": []string{"India"},
							},
						},
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"terms":{"country":["India"]}},"size":3,"sort":[{"original_title.raw":{"order":"desc"}}]}
`)
	})
}

func TestBasicDataSearchWithCustomQuery(t *testing.T) {
	Convey("with custom query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      10,
					"dataField": []string{"original_title", "original_title.search"},
					"customQuery": map[string]interface{}{
						"query": map[string]interface{}{
							"terms": map[string]interface{}{
								"country": []string{"india"},
							},
						},
					},
					"value":   "india",
					"execute": false,
				},
				{
					"id":        "SearchResult",
					"size":      10,
					"dataField": []string{"original_title"},
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
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":{"terms":{"country":["india"]}}}}]}},"size":10}
`)
	})
}

func TestBasicDataSearchWithCustomQueryWithoutField(t *testing.T) {
	Convey("without dataField and with custom query", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":   "BookSensor",
					"size": 10,
					"customQuery": map[string]interface{}{
						"query": map[string]interface{}{
							"terms": map[string]interface{}{
								"country": []string{"india"},
							},
						},
					},
					"value":   "india",
					"execute": false,
				},
				{
					"id":   "SearchResult",
					"size": 10,
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
		So(transformedQuery, ShouldResemble, `{"preference":"SearchResult_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":{"terms":{"country":["india"]}}}}]}},"size":10}
`)
	})
}

func TestQueryWithReact(t *testing.T) {
	Convey("with and & or clauses", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"dataField": []string{"original_title"},
					"value":     "batman",
					"size":      1,
					"execute":   false,
				},
				{
					"id":              "AuthorFilter",
					"type":            "term",
					"dataField":       []string{"genres_new_data.keyword"},
					"aggregationSize": 5,
					"size":            0,
					"value": []string{
						"Romance",
					},
					"execute": false,
				},
				{
					"id": "Results",
					"react": map[string]interface{}{
						"and": []string{"BookSensor"},
						"or":  []string{"AuthorFilter"},
					},
					"dataField": []interface{}{
						map[string]interface{}{
							"field":  "original_title.keyword",
							"weight": 1,
						},
					},
					"sortBy": "asc",
					"size":   5,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		So(transformedQuery, ShouldResemble, `{"preference":"Results_127.0.0.1"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"must":[{"bool":{"must":[{"bool":{"minimum_should_match":1,"should":[{"multi_match":{"fields":["original_title"],"operator":"or","query":"batman","type":"cross_fields"}},{"multi_match":{"fields":["original_title"],"fuzziness":0,"operator":"or","query":"batman","type":"best_fields"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"batman","type":"phrase"}},{"multi_match":{"fields":["original_title"],"operator":"or","query":"batman","type":"phrase_prefix"}}]}}]}},{"bool":{"minimum_should_match":1,"should":[{"bool":{"should":[{"terms":{"genres_new_data.keyword":["Romance"]}}]}}]}}]}},"size":5,"sort":[{"original_title.keyword":{"order":"asc"}}]}
`)
	})
}
