package querytranslate

import (
	"github.com/smartystreets/goconvey/convey"
	"testing"
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
