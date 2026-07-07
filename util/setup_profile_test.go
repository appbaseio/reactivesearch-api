package util

import (
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSetupProfile(t *testing.T) {
	Convey("Setup profile", t, func() {
		defer os.Unsetenv(envSetupProfile)

		Convey("defaults to full when unset", func() {
			So(GetSetupProfile(), ShouldEqual, SetupProfileFull)
		})

		Convey("minimal creates only core indexes", func() {
			os.Setenv(envSetupProfile, "minimal")
			So(ShouldCreateMetaIndex(MetaIndexUsers), ShouldBeTrue)
			So(ShouldCreateMetaIndex(MetaIndexPipelines), ShouldBeTrue)
			So(ShouldCreateMetaIndex(MetaIndexPipelineVars), ShouldBeTrue)
			So(ShouldCreateMetaIndex(MetaIndexPublicKey), ShouldBeFalse)
			So(ShouldCreateMetaIndex(MetaIndexLogs), ShouldBeFalse)
			So(ShouldCreateMetaIndex(MetaIndexAnalytics), ShouldBeFalse)
			So(ShouldCreateMetaIndex(MetaIndexRules), ShouldBeFalse)
		})

		Convey("standard adds observability indexes but not full-only", func() {
			os.Setenv(envSetupProfile, "standard")
			So(ShouldCreateMetaIndex(MetaIndexSynonyms), ShouldBeTrue)
			So(ShouldCreateMetaIndex(MetaIndexAnalytics), ShouldBeTrue)
			So(ShouldCreateMetaIndex(MetaIndexAnalyticsInsights), ShouldBeFalse)
			So(ShouldCreateMetaIndex(MetaIndexRecentDocuments), ShouldBeFalse)
			So(ShouldCreateMetaIndex(MetaIndexRules), ShouldBeFalse)
			So(ShouldCreateMetaIndex(MetaIndexCache), ShouldBeFalse)
		})

		Convey("full creates everything", func() {
			os.Setenv(envSetupProfile, "full")
			So(ShouldCreateMetaIndex(MetaIndexRules), ShouldBeTrue)
			So(ShouldCreateMetaIndex(MetaIndexRecentDocuments), ShouldBeTrue)
			So(ShouldCreateMetaIndex(MetaIndexPublicKey), ShouldBeTrue)
		})

		Convey("MetaIndexShards uses 1 outside full profile", func() {
			os.Setenv(envSetupProfile, "standard")
			So(MetaIndexShards(3), ShouldEqual, 1)
			os.Setenv(envSetupProfile, "full")
			So(MetaIndexShards(3), ShouldEqual, 3)
		})
	})
}
