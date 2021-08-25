package util

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateIndex(t *testing.T) {
	Convey("ValidateIndex", t, func() {
		Convey("* pattern", func() {
			m, _ := ValidateIndex("dede", "*")
			So(m, ShouldBeTrue)
		})
		Convey("* in between (success)", func() {
			m, _ := ValidateIndex("test3m", "test*m")
			So(m, ShouldBeTrue)
		})
		Convey("* in between (failure)", func() {
			m, _ := ValidateIndex("test3mn", "test*m")
			So(m, ShouldBeFalse)
		})
		Convey("* at end (success)", func() {
			m, _ := ValidateIndex("test2", "test*")
			So(m, ShouldBeTrue)
		})
		Convey("* at end (failure)", func() {
			m, _ := ValidateIndex("def", "test*")
			So(m, ShouldBeFalse)
		})
		Convey("* at start (success)", func() {
			m, _ := ValidateIndex("pl_test", "*test")
			So(m, ShouldBeTrue)
		})
		Convey("* at start (failure)", func() {
			m, _ := ValidateIndex("pl_ded", "*test")
			So(m, ShouldBeFalse)
		})

		Convey("Exact Match (success)", func() {
			m, _ := ValidateIndex("test", "test")
			So(m, ShouldBeTrue)
		})
		Convey("Exact Match (failure)", func() {
			m, _ := ValidateIndex("testt", "test")
			So(m, ShouldBeFalse)
		})
	})
}
