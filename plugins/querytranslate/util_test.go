package querytranslate

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNormalizedDataFields(t *testing.T) {
	Convey("dataField as string", t, func() {
		So(NormalizedDataFields("title", []float64{}), ShouldResemble, []DataField{
			{
				Field: "title",
			},
		})
	})
	Convey("dataField with integer field weight", t, func() {
		So(NormalizedDataFields(map[string]interface{}{
			"field":  "title",
			"weight": 5,
		}, []float64{}), ShouldResemble, []DataField{
			{
				Field:  "title",
				Weight: 5,
			},
		})
	})
	Convey("dataField with float field weight", t, func() {
		So(NormalizedDataFields(map[string]interface{}{
			"field":  "title",
			"weight": 0.8,
		}, []float64{}), ShouldResemble, []DataField{
			{
				Field:  "title",
				Weight: 0.80,
			},
		})
	})
	Convey("dataField without weight", t, func() {
		So(NormalizedDataFields(map[string]interface{}{
			"field": "title",
		}, []float64{}), ShouldResemble, []DataField{
			{
				Field: "title",
			},
		})
	})
	Convey("dataField as an array of string", t, func() {
		So(NormalizedDataFields([]interface{}{"title", "description"}, []float64{}), ShouldResemble, []DataField{
			{
				Field: "title",
			},
			{
				Field: "description",
			},
		})
	})
	Convey("dataField as an array of fields with weights", t, func() {
		So(NormalizedDataFields([]interface{}{
			map[string]interface{}{
				"field":  "title",
				"weight": 5,
			},
			map[string]interface{}{
				"field":  "description",
				"weight": 0.8,
			},
		}, []float64{}), ShouldResemble,
			[]DataField{
				{
					Field:  "title",
					Weight: 5,
				},
				{
					Field:  "description",
					Weight: 0.80,
				},
			})
	})
	Convey("dataField as an array of fields with/without weights", t, func() {
		So(NormalizedDataFields([]interface{}{
			map[string]interface{}{
				"field":  "title",
				"weight": 5,
			},
			map[string]interface{}{
				"field":  "description",
				"weight": 0.8,
			},
			"authors",
		}, []float64{}), ShouldResemble,
			[]DataField{
				{
					Field:  "title",
					Weight: 5,
				},
				{
					Field:  "description",
					Weight: 0.80,
				},
				{
					Field: "authors",
				},
			})
	})
	Convey("dataField as an array of strings with field weights", t, func() {
		So(NormalizedDataFields([]string{"title", "description"}, []float64{0.83, 0.23}), ShouldResemble,
			[]DataField{
				{
					Field:  "title",
					Weight: 0.83,
				},
				{
					Field:  "description",
					Weight: 0.23,
				},
			})
	})
}

func TestGetSizeFromQuery(t *testing.T) {
	Convey("empty map should return nil", t, func() {
		query := map[string]interface{}{}
		So(getSizeFromQuery(&query, "size"), ShouldBeNil)
	})
	Convey("key at first level", t, func() {
		query := map[string]interface{}{
			"size": 10,
		}
		value := getSizeFromQuery(&query, "size")
		So(*value, ShouldResemble, 10)
	})
	Convey("key at nested level", t, func() {
		query := map[string]interface{}{
			"aggs": map[string]interface{}{
				"product": map[string]interface{}{
					"terms": map[string]interface{}{},
					"size":  10,
				},
			},
		}
		value := getSizeFromQuery(&query, "size")
		So(*value, ShouldResemble, 10)
	})
}
