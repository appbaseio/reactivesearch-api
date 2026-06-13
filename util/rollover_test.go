package util

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRolloverConditionsFromMap(t *testing.T) {
	Convey("RolloverConditionsFromMap extracts thresholds", t, func() {
		conditions := map[string]interface{}{
			"max_age":  "7d",
			"max_docs": float64(10000),
			"max_size": "1gb",
		}
		parsed := RolloverConditionsFromMap(conditions)
		So(parsed.MaxAge, ShouldEqual, "7d")
		So(parsed.MaxDocs, ShouldEqual, 10000)
		So(parsed.MaxSize, ShouldEqual, "1gb")
	})
}

func TestParseESDuration(t *testing.T) {
	Convey("parseESDuration handles Elasticsearch units", t, func() {
		d, err := parseESDuration("7d")
		So(err, ShouldBeNil)
		So(d.Hours(), ShouldEqual, 7*24)

		d, err = parseESDuration("30d")
		So(err, ShouldBeNil)
		So(d.Hours(), ShouldEqual, 30*24)

		d, err = parseESDuration("500ms")
		So(err, ShouldBeNil)
		So(d.Milliseconds(), ShouldEqual, 500)
	})
}

func TestParseStoreSize(t *testing.T) {
	Convey("parseStoreSize handles human-readable and byte values", t, func() {
		size, err := parseStoreSize("1gb")
		So(err, ShouldBeNil)
		So(size, ShouldEqual, 1024*1024*1024)

		size, err = parseStoreSize("1048576")
		So(err, ShouldBeNil)
		So(size, ShouldEqual, 1048576)

		size, err = parseStoreSize("-")
		So(err, ShouldBeNil)
		So(size, ShouldEqual, 0)
	})
}
