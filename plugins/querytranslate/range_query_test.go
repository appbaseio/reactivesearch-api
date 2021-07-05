package querytranslate

import (
	"github.com/smartystreets/goconvey/convey"
	"testing"
)

func TestMultiRangeWithValue(t *testing.T) {
	convey.Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      3,
					"dataField": []string{"average_rating"},
					"type":      "range",
					"value": []map[string]interface{}{{
						"start": 0,
						"end":   3,
						"boost": 2,
					}, {
						"start": 3,
						"end":   4,
						"boost": 2,
					}},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"boost":1,"minimum_should_match":1,"should":[{"range":{"average_rating":{"boost":2,"gte":0,"lte":3}}},{"range":{"average_rating":{"boost":2,"gte":3,"lte":4}}}]}},"size":3}
`)
	})
}

func TestRangeSliderWithValue(t *testing.T) {
	convey.Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      3,
					"dataField": []string{"ratings_count"},
					"type":      "range",
					"value": map[string]interface{}{
						"start": 3000,
						"end":   50000,
						"boost": 2,
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"range":{"ratings_count":{"boost":2,"gte":3000,"lte":50000}}},"size":3}
`)
	})
}

func TestRangeSliderWithNullValues(t *testing.T) {
	convey.Convey("with includeNullValues", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      3,
					"dataField": []string{"ratings_count"},
					"type":      "range",
					"value": map[string]interface{}{
						"start": 3000,
						"end":   50000,
						"boost": 2,
					},
					"includeNullValues": true,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"bool":{"should":[{"range":{"ratings_count":{"boost":2,"gte":3000,"lte":50000}}},{"bool":{"must_not":{"exists":{"field":"ratings_count"}}}}]}},"size":3}
`)
	})
}

func TestRangeSliderWithNestedField(t *testing.T) {
	convey.Convey("with nested field", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      3,
					"dataField": []string{"ratings_count"},
					"type":      "range",
					"value": map[string]interface{}{
						"start": 3000,
						"end":   50000,
						"boost": 2,
					},
					"nestedField": "ratings_count.raw",
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"nested":{"path":"ratings_count.raw","query":{"range":{"ratings_count":{"boost":2,"gte":3000,"lte":50000}}}}},"size":3}
`)
	})
}

func TestRangeSliderWithHistogram(t *testing.T) {
	convey.Convey("with histogram", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "BookSensor",
					"size":      3,
					"dataField": []string{"ratings_count"},
					"type":      "range",
					"value": map[string]interface{}{
						"start": 3000,
						"end":   50000,
						"boost": 2,
					},
					"nestedField":  "ratings_count.raw",
					"aggregations": []string{"histogram"},
					"interval":     470,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"BookSensor"}
{"_source":{"excludes":[],"includes":["*"]},"aggs":{"ratings_count":{"histogram":{"field":"ratings_count","interval":470,"offset":3000}}},"query":{"nested":{"path":"ratings_count.raw","query":{"range":{"ratings_count":{"boost":2,"gte":3000,"lte":50000}}}}},"size":3}
`)
	})
}

func TestDatePicker(t *testing.T) {
	convey.Convey("with date picker", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "DateSensor",
					"size":      40,
					"dataField": []string{"date_from"},
					"type":      "range",
					"value": map[string]interface{}{
						"start": "20170510",
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"DateSensor"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"range":{"date_from":{"gte":"20170510"}}},"size":40}
`)
	})
}

func TestDateRangePicker(t *testing.T) {
	convey.Convey("with date range picker", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "DateSensor",
					"size":      40,
					"dataField": []string{"date_from"},
					"type":      "range",
					"value": map[string]interface{}{
						"start": "20170515",
					},
				},
				{
					"id":        "DateSensor",
					"size":      40,
					"dataField": []string{"date_to"},
					"type":      "range",
					"value": map[string]interface{}{
						"end": "20170518",
					},
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"DateSensor"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"range":{"date_from":{"gte":"20170515"}}},"size":40}
{"preference":"DateSensor"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"range":{"date_to":{"lte":"20170518"}}},"size":40}
`)
	})
}
