package util

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBilling(t *testing.T) {
	Convey("Billing", t, func() {
		Convey("Set Tier", func() {
			var plan = Sandbox
			SetTier(&plan)
			So(GetTier(nil).String(), ShouldEqual, Sandbox.String())
		})
		Convey("Set TimeValidity", func() {
			var timeValidityMock = 1200000
			SetTimeValidity(int64(timeValidityMock))
			So(GetTimeValidity(), ShouldEqual, timeValidityMock)
		})
		Convey("Set FeatureCustomEvents", func() {
			SetFeatureCustomEvents(true)
			So(GetFeatureCustomEvents(), ShouldEqual, true)
		})
		Convey("Set FeatureSuggestions", func() {
			SetFeatureSuggestions(true)
			So(GetFeatureSuggestions(), ShouldEqual, true)
		})
		Convey("Set FeatureRules", func() {
			SetFeatureRules(true)
			So(GetFeatureRules(), ShouldEqual, true)
		})
		Convey("Set FeatureSearchRelevancy", func() {
			SetFeatureSearchRelevancy(true)
			So(GetFeatureSearchRelevancy(), ShouldEqual, true)
		})
		Convey("Set FeatureSearchGrader", func() {
			SetFeatureSearchGrader(true)
			So(GetFeatureSearchGrader(), ShouldEqual, true)
		})
		Convey("Set FeatureEcommerce", func() {
			SetFeatureEcommerce(true)
			So(GetFeatureEcommerce(), ShouldEqual, true)
		})
		Convey("Set FeatureUIBuilder", func() {
			SetFeatureUIBuilderPremium(true)
			So(GetFeatureUIBuilderPremium(), ShouldEqual, true)
		})
		Convey("Set FeatureCache", func() {
			SetFeatureCache(true)
			So(GetFeatureCache(), ShouldEqual, true)
		})
		Convey("Set FeaturePipelines", func() {
			SetFeaturePipelines(true)
			So(GetFeaturePipelines(), ShouldEqual, true)
		})
		Convey("Validate TimeValidity: Positive Value", func() {
			// Set TimeValidity to a positive value
			var timeValidityMock = 1200000
			SetTimeValidity(int64(timeValidityMock))
			So(true, ShouldEqual, validateTimeValidity(int64(timeValidityMock)))
		})
		Convey("Validate TimeValidity: Negative value greater than 168 hours", func() {
			// Set TimeValidity to a positive value
			var timeValidityMock = -(3600*168 + 10)
			SetTimeValidity(int64(timeValidityMock))
			So(false, ShouldEqual, validateTimeValidity(int64(timeValidityMock)))
		})
		Convey("Validate TimeValidity: Negative value less than 168 hours", func() {
			// Set TimeValidity to a positive value
			var timeValidityMock = -(3600*168 - 10)
			SetTimeValidity(int64(timeValidityMock))
			So(true, ShouldEqual, validateTimeValidity(int64(timeValidityMock)))
		})
	})
}
