package reindex

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestParseStoreSize(t *testing.T) {
	Convey("ParseStoreSize", t, func() {
		Convey("should treat empty and dash as zero", func() {
			size, err := ParseStoreSize("")
			So(err, ShouldBeNil)
			So(size, ShouldEqual, 0)

			size, err = ParseStoreSize("-")
			So(err, ShouldBeNil)
			So(size, ShouldEqual, 0)
		})

		Convey("should parse human-readable sizes", func() {
			size, err := ParseStoreSize("1.2kb")
			So(err, ShouldBeNil)
			So(size, ShouldEqual, int64(1228))

			size, err = ParseStoreSize("100mb")
			So(err, ShouldBeNil)
			So(size, ShouldEqual, int64(100*1024*1024))

			size, err = ParseStoreSize("2gb")
			So(err, ShouldBeNil)
			So(size, ShouldEqual, int64(2*1024*1024*1024))
		})

		Convey("should parse plain byte counts", func() {
			size, err := ParseStoreSize("512")
			So(err, ShouldBeNil)
			So(size, ShouldEqual, 512)

			size, err = ParseStoreSize("100b")
			So(err, ShouldBeNil)
			So(size, ShouldEqual, 100)
		})

		Convey("should reject invalid values", func() {
			_, err := ParseStoreSize("not-a-size")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestIsNumericStoreSize(t *testing.T) {
	Convey("isNumericStoreSize", t, func() {
		So(isNumericStoreSize("12345"), ShouldBeTrue)
		So(isNumericStoreSize("1.2kb"), ShouldBeFalse)
		So(isNumericStoreSize(""), ShouldBeFalse)
	})
}
