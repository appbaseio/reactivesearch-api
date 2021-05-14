package sourcefilter

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestApplySourceFiltering(t *testing.T) {
	Convey("Should return the exact source if include or exclude properties are empty", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": "value1",
		}, []string{}, []string{})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": "value1",
		})
	})
	Convey("Filter by exact name (include)", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		}, []string{"key1"}, []string{})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": "value1",
		})
	})
	Convey("Filter by exact name (exclude)", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		}, []string{}, []string{"key1"})
		So(output, ShouldResemble, map[string]interface{}{
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		})
	})
	Convey("Filter by exact name (both), multiple values", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		}, []string{"key1", "key2", "key3"}, []string{"key1"})
		So(output, ShouldResemble, map[string]interface{}{
			"key2": "value2",
			"key3": "value3",
		})
	})
	Convey("Filter by exact name (both), exclude must have priority for matching keys", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
			"key4": "value4",
		}, []string{"key1"}, []string{"key1"})
		So(output, ShouldResemble, nil)
	})
	Convey("Filter by exact name: nested object", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": map[string]interface{}{
				"key11": "value11",
				"key12": "value12",
				"key13": "value13",
			},
		}, []string{"key1.key11"}, []string{})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": map[string]interface{}{
				"key11": "value11",
			},
		})
	})
	Convey("Filter by exact name: nested object advanced", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": []interface{}{
				map[string]interface{}{
					"key11": "value11",
					"key12": "value12",
				},
				map[string]interface{}{
					"key11": "value11",
					"key12": "value12",
				},
				map[string]interface{}{
					"key11": "value11",
					"key12": "value12",
				},
			},
		}, []string{"key1.key11"}, []string{})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": []interface{}{
				map[string]interface{}{
					"key11": "value11",
				},
				map[string]interface{}{
					"key11": "value11",
				},
				map[string]interface{}{
					"key11": "value11",
				},
			},
		})
	})
	Convey("Filter by pattern: include", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": []interface{}{"value11", "value12", "value13"},
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		}, []string{"*.key221"}, []string{})
		So(output, ShouldResemble, map[string]interface{}{
			"key2": map[string]interface{}{
				"key22": map[string]interface{}{
					"key221": "value221",
				},
			},
		})
	})
	Convey("Filter by pattern: exclude", t, func() {
		output := ApplySourceFiltering(map[string]interface{}{
			"key1": []interface{}{"value11", "value12", "value13"},
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		}, []string{}, []string{"*.key221"})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": []interface{}{"value11", "value12", "value13"},
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key222": "value222",
				},
			},
		})
	})
}

func TestDotNotate(t *testing.T) {
	Convey("Dot notation for simple nesting (only map)", t, func() {
		output := dotNotate(map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		}, nil, "")
		So(output, ShouldResemble, map[string]interface{}{
			"key1":              "value1",
			"key2.key21":        "value21",
			"key2.key22.key221": "value221",
			"key2.key22.key222": "value222",
		})
	})
	Convey("Dot notation for simple nesting (array)", t, func() {
		output := dotNotate(map[string]interface{}{
			"key1": []interface{}{"value11", "value12", "value13"},
		}, nil, "")
		So(output, ShouldResemble, map[string]interface{}{
			"key1.0": "value11",
			"key1.1": "value12",
			"key1.2": "value13",
		})
	})
	Convey("advanced nesting", t, func() {
		output := dotNotate(map[string]interface{}{
			"key1": []interface{}{"value11", "value12", "value13"},
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
				"key33": map[string]interface{}{
					"key331": []interface{}{
						map[string]interface{}{
							"key3311": "value3311",
						},
						map[string]interface{}{
							"key3312": "value3312",
						},
					},
					"key332": "value332",
				},
				"key44": []interface{}{
					[]interface{}{"key445"},
					[]interface{}{"key446", "key447"},
				},
			},
		}, nil, "")
		So(output, ShouldResemble, map[string]interface{}{
			"key1.0":                      "value11",
			"key1.1":                      "value12",
			"key1.2":                      "value13",
			"key2.key21":                  "value21",
			"key2.key22.key221":           "value221",
			"key2.key22.key222":           "value222",
			"key2.key33.key331.0.key3311": "value3311",
			"key2.key33.key331.1.key3312": "value3312",
			"key2.key33.key332":           "value332",
			"key2.key44.0.0":              "key445",
			"key2.key44.1.0":              "key446",
			"key2.key44.1.1":              "key447",
		})
	})
}

func TestDotNotationToMap(t *testing.T) {
	Convey("Dot notation to map for simple nesting (only map)", t, func() {
		output := dotNotationToMap(map[string]interface{}{
			"key1":              "value1",
			"key2.key21":        "value21",
			"key2.key22.key221": "value221",
			"key2.key22.key222": "value222",
		})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		})
	})
	Convey("Dot notation to map for simple nesting (array)", t, func() {
		output := dotNotationToMap(map[string]interface{}{
			"key1.0": "value11",
			"key1.1": "value12",
			"key1.2": "value13",
		})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": []interface{}{"value11", "value12", "value13"},
		})
	})
	Convey("advanced nesting", t, func() {
		output := dotNotationToMap(map[string]interface{}{
			"key1.0":                      "value11",
			"key1.1":                      "value12",
			"key1.2":                      "value13",
			"key2.key21":                  "value21",
			"key2.key22.key221":           "value221",
			"key2.key22.key222":           "value222",
			"key2.key33.key331.0.key3311": "value3311",
			"key2.key33.key331.1.key3312": "value3312",
			"key2.key33.key332":           "value332",
			"key2.key44.0.0":              "key445",
			"key2.key44.1.0":              "key446",
			"key2.key44.1.1":              "key447",
		})
		So(output, ShouldResemble, map[string]interface{}{
			"key1": []interface{}{"value11", "value12", "value13"},
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
				"key33": map[string]interface{}{
					"key331": []interface{}{
						map[string]interface{}{
							"key3311": "value3311",
						},
						map[string]interface{}{
							"key3312": "value3312",
						},
					},
					"key332": "value332",
				},
				"key44": []interface{}{
					[]interface{}{"key445"},
					[]interface{}{"key446", "key447"},
				},
			},
		})
	})

}

func TestGenerateMap(t *testing.T) {
	Convey("Basic Test", t, func() {
		output := generateMap(map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		}, "key2.key11", "value11")
		So(output, ShouldResemble, map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key11": "value11",
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		})
	})
	Convey("Basic Test (with array)", t, func() {
		output := generateMap(map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		}, "key3.0", "value13")
		So(output, ShouldResemble, map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
			"key3": []interface{}{"value13"},
		})
	})
	Convey("Advanced Test (with array)", t, func() {
		output := generateMap(map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
			},
		}, "key2.key33.key331.0.key3311", "value13")
		So(output, ShouldResemble, map[string]interface{}{
			"key1": "value1",
			"key2": map[string]interface{}{
				"key21": "value21",
				"key22": map[string]interface{}{
					"key221": "value221",
					"key222": "value222",
				},
				"key33": map[string]interface{}{
					"key331": []interface{}{map[string]interface{}{
						"key3311": "value13",
					}},
				},
			},
		})
	})
}
