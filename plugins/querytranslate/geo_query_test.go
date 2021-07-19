package querytranslate

import (
	"testing"

	"github.com/smartystreets/goconvey/convey"
)

func TestGeoDropdownWithValue(t *testing.T) {
	convey.Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "GeoDistanceDropdown",
					"dataField": []string{"location"},
					"type":      "geo",
					"value": map[string]interface{}{
						"distance": 10,
						"unit":     "mi",
						"location": "51.5073509, -0.1277583",
					},
					"size": 100,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"GeoDistanceDropdown"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"geo_distance":{"distance":"10mi","location":"51.5073509, -0.1277583"}},"size":100}
`)
	})
}

func TestGeoWithNoDataField(t *testing.T) {
	convey.Convey("should not throw error when value is not defined and `dataField` is not defined and `react` property is not defined and `defaultQuery` is not defined", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":   "BookSensor",
					"type": "geo",
				},
			},
		}
		_, err := transformQuery(query)
		convey.So(err, convey.ShouldBeNil)
	})
}

func TestGeoDistanceSliderWithValue(t *testing.T) {
	convey.Convey("with value", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":        "GeoDistanceSlider",
					"dataField": []string{"location"},
					"type":      "geo",
					"value": map[string]interface{}{
						"distance": 10,
						"unit":     "mi",
						"location": "51.5073509, -0.1277583",
					},
					"size": 100,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"GeoDistanceSlider"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"geo_distance":{"distance":"10mi","location":"51.5073509, -0.1277583"}},"size":100}
`)
	})
}

func TestGeoDistanceSliderWithNestedField(t *testing.T) {
	convey.Convey("with nested field", t, func() {
		query := map[string]interface{}{
			"query": []map[string]interface{}{
				{
					"id":          "GeoDistanceSlider",
					"dataField":   []string{"location"},
					"nestedField": "location.raw",
					"type":        "geo",
					"value": map[string]interface{}{
						"distance": 10,
						"unit":     "mi",
						"location": "51.5073509, -0.1277583",
					},
					"size": 100,
				},
			},
		}
		transformedQuery, err := transformQuery(query)
		if err != nil {
			t.Fatalf("Test Failed %v instead\n", err)
		}
		convey.So(transformedQuery, convey.ShouldResemble, `{"preference":"GeoDistanceSlider"}
{"_source":{"excludes":[],"includes":["*"]},"query":{"nested":{"path":"location.raw","query":{"geo_distance":{"distance":"10mi","location":"51.5073509, -0.1277583"}}}},"size":100}
`)
	})
}
