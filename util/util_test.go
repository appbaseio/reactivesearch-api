package util

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateIndex(t *testing.T) {
	Convey("ValidateIndex", t, func() {
		Convey("* pattern", func() {
			m, _ := ValidateIndex("*", "dede")
			So(m, ShouldBeTrue)
		})
		Convey("* in between (success)", func() {
			m, _ := ValidateIndex("test*m", "test3m")
			So(m, ShouldBeTrue)
		})
		Convey("* in between (failure)", func() {
			m, _ := ValidateIndex("test*m", "test3mn")
			So(m, ShouldBeFalse)
		})
		Convey("* at end (success)", func() {
			m, _ := ValidateIndex("test*", "test2")
			So(m, ShouldBeTrue)
		})
		Convey("* at end (failure)", func() {
			m, _ := ValidateIndex("test*", "def")
			So(m, ShouldBeFalse)
		})
		Convey("* at start (success)", func() {
			m, _ := ValidateIndex("*test", "pl_test")
			So(m, ShouldBeTrue)
		})
		Convey("* at start (failure)", func() {
			m, _ := ValidateIndex("*test", "pl_ded")
			So(m, ShouldBeFalse)
		})

		Convey("Exact Match (success)", func() {
			m, _ := ValidateIndex("test", "test")
			So(m, ShouldBeTrue)
		})
		Convey("Exact Match start (failure)", func() {
			m, _ := ValidateIndex("test", "testt")
			So(m, ShouldBeFalse)
		})
		Convey("Exact Match end (failure)", func() {
			m, _ := ValidateIndex("test", "ttest")
			So(m, ShouldBeFalse)
		})
	})
}
