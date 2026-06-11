package util

import (
	"os"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestMetaIndexName(t *testing.T) {
	Convey("With RS_META_INDEX_PREFIX set", t, func() {
		os.Setenv(envMetaIndexPrefix, "rs_")
		defer os.Unsetenv(envMetaIndexPrefix)

		Convey("dot-prefixed defaults are rewritten", func() {
			So(MetaIndexName(".pipelines"), ShouldEqual, "rs_pipelines")
			So(MetaIndexName(".pipeline_logs"), ShouldEqual, "rs_pipeline_logs")
			So(MetaIndexName(".rs-synonyms"), ShouldEqual, "rs_rs-synonyms")
		})

		Convey("non-dot names are returned unchanged", func() {
			So(MetaIndexName("custom_index"), ShouldEqual, "custom_index")
			So(MetaIndexName("rs_pipelines"), ShouldEqual, "rs_pipelines")
		})
	})

	Convey("With a custom prefix", t, func() {
		os.Setenv(envMetaIndexPrefix, "meta_")
		defer os.Unsetenv(envMetaIndexPrefix)

		So(MetaIndexName(".users"), ShouldEqual, "meta_users")
	})
}

func TestAdaptIndexBodyParsing(t *testing.T) {
	Convey("stripUnsupportedSettings removes serverless-rejected settings", t, func() {
		settings := map[string]interface{}{
			"index.hidden":             true,
			"index.number_of_shards":   3,
			"index.number_of_replicas": 1,
			"index.max_result_window":  100000,
		}
		stripUnsupportedSettings(settings)
		So(settings, ShouldNotContainKey, "index.hidden")
		So(settings, ShouldNotContainKey, "index.number_of_shards")
		So(settings, ShouldNotContainKey, "index.number_of_replicas")
		So(settings, ShouldContainKey, "index.max_result_window")
	})
}
