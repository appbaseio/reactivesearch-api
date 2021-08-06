package telemetry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetClientIP4(t *testing.T) {
	Convey("should be empty with ipv6", t, func() {
		So(getClientIP4("2001:0db8:85a3:0000:0000:8a2e:0370:7334"), ShouldResemble, "")
	})
	Convey("with ipv4", t, func() {
		So(getClientIP4("198.128.2.4"), ShouldResemble, "198.128.2.x")
	})
}

func TestGetClientIP6(t *testing.T) {
	Convey("should be empty with ipv4", t, func() {
		So(getClientIP6("198.128.2.4"), ShouldResemble, "")
	})
	Convey("with ipv4 basic", t, func() {
		So(getClientIP6("2001:db8:85a3::8a2e:370:7334"), ShouldResemble, "2001:db8:85a3::8a2e:370:x")
	})
	Convey("with ipv4 short", t, func() {
		So(getClientIP6("::1"), ShouldResemble, "::x")
	})
}
