package storedquery

import (
	"encoding/json"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFlattenReact(t *testing.T) {
	Convey("Basic: string", t, func() {
		So(flattenReact("c_1"), ShouldResemble, []string{"c_1"})
	})
	Convey("Basic: array of strings", t, func() {
		So(flattenReact([]interface{}{"c_1", "c_2"}), ShouldResemble, []string{"c_1", "c_2"})
	})
	Convey("Basic: nested", t, func() {
		So(flattenReact(map[string]interface{}{
			"and": []interface{}{"c_1", "c_2"},
			"or":  []interface{}{"c_3", "c_4", []interface{}{"c_5", "c_6"}},
		}), ShouldResemble, []string{"c_1", "c_2", "c_3", "c_4", "c_5", "c_6"})
	})
}

func TestRenderQuery(t *testing.T) {
	Convey("Basic: string value", t, func() {
		query := map[string]interface{}{
			"terms": map[string]interface{}{
				"brand": "{{brand}}",
			},
		}
		params := map[string]interface{}{
			"brand": "puma",
		}
		outputQuery := map[string]interface{}{
			"terms": map[string]interface{}{
				"brand": "puma",
			},
		}
		outputQueryBytes, _ := json.Marshal(outputQuery)
		queryBytes, _ := json.Marshal(query)
		parsedQuery, _ := renderQuery(queryBytes, params)
		So(parsedQuery, ShouldResemble, string(outputQueryBytes))
	})
	Convey("Basic: string key and value", t, func() {
		query := map[string]interface{}{
			"terms": map[string]interface{}{
				"{{field}}": "{{value}}",
			},
		}
		params := map[string]interface{}{
			"field": "brand",
			"value": "puma",
		}
		outputQuery := map[string]interface{}{
			"terms": map[string]interface{}{
				"brand": "puma",
			},
		}
		outputQueryBytes, _ := json.Marshal(outputQuery)
		queryBytes, _ := json.Marshal(query)
		parsedQuery, _ := renderQuery(queryBytes, params)
		So(parsedQuery, ShouldResemble, string(outputQueryBytes))
	})
	Convey("Basic: Array", t, func() {
		query := map[string]interface{}{
			"terms": map[string]interface{}{
				"brand": "{{brand}}",
			},
		}
		params := map[string]interface{}{
			"brand": []interface{}{"puma", "nike"},
		}
		outputQuery := map[string]interface{}{
			"terms": map[string]interface{}{
				"brand": []interface{}{"puma", "nike"},
			},
		}
		outputQueryBytes, _ := json.Marshal(outputQuery)
		queryBytes, _ := json.Marshal(query)
		parsedQuery, _ := renderQuery(queryBytes, params)
		So(parsedQuery, ShouldResemble, string(outputQueryBytes))
	})
	Convey("Basic: string key and float/int value", t, func() {
		query := map[string]interface{}{
			"terms": map[string]interface{}{
				"{{field}}": "{{value}}",
			},
		}
		params := map[string]interface{}{
			"field": "brand",
			"value": 5,
		}
		outputQuery := map[string]interface{}{
			"terms": map[string]interface{}{
				"brand": 5,
			},
		}
		outputQueryBytes, _ := json.Marshal(outputQuery)
		queryBytes, _ := json.Marshal(query)
		parsedQuery, _ := renderQuery(queryBytes, params)
		So(parsedQuery, ShouldResemble, string(outputQueryBytes))
	})
	Convey("Basic: Map", t, func() {
		query := map[string]interface{}{
			"geo": map[string]interface{}{
				"location": "{{location}}",
			},
		}
		params := map[string]interface{}{
			"location": map[string]interface{}{
				"lat": 23,
				"lng": 77,
			},
		}
		outputQuery := map[string]interface{}{
			"geo": map[string]interface{}{
				"location": map[string]interface{}{
					"lat": 23,
					"lng": 77,
				},
			},
		}
		outputQueryBytes, _ := json.Marshal(outputQuery)
		queryBytes, _ := json.Marshal(query)
		parsedQuery, _ := renderQuery(queryBytes, params)
		var parsedQueryMap map[string]interface{}
		json.Unmarshal([]byte(parsedQuery), &parsedQueryMap)
		So(parsedQuery, ShouldResemble, string(outputQueryBytes))
	})
}
