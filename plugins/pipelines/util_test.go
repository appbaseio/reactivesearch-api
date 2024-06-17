package pipelines

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGetDeepDependencyMap(t *testing.T) {
	Convey("Basic deep dependencies", t, func() {
		dependencies := getDependencyMap(map[string]StageStatus{
			"a": {Needs: []string{"b"}},
			"b": {Needs: []string{"c"}},
			"c": {Needs: []string{}},
		})
		So(dependencies, ShouldResemble, map[string]map[string]bool{
			"a": {
				"b": true,
				"c": true,
			},
			"b": {
				"c": true,
			},
		})
	})
	Convey("Direct cyclic dependency", t, func() {
		dependencies := getDependencyMap(map[string]StageStatus{
			"a": {Needs: []string{"a", "b"}},
			"b": {Needs: []string{"c"}},
			"c": {Needs: []string{}},
		})
		So(dependencies, ShouldResemble, map[string]map[string]bool{
			"a": {
				"a": true,
				"b": true,
				"c": true,
			},
			"b": {
				"c": true,
			},
		})
	})
	Convey("In-Direct cyclic dependency", t, func() {
		dependencies := getDependencyMap(map[string]StageStatus{
			"a": {Needs: []string{"b"}},
			"b": {Needs: []string{"a"}},
		})
		So(dependencies, ShouldResemble, map[string]map[string]bool{
			"a": {
				"a": true,
				"b": true,
			},
			"b": {
				"a": true,
				"b": true,
			},
		})
	})
}

func TestGetDependencyMap(t *testing.T) {
	Convey("throw error when cyclic dependency found", t, func() {
		stageId1 := "a"
		stageId1Needs := []string{"b"}
		stageId2 := "b"
		stageId2Needs := []string{"a"}
		_, err := getStageStatusMap([]ESPipelineStage{
			{
				ID:    &stageId1,
				Needs: &stageId1Needs,
			},
			{
				ID:    &stageId2,
				Needs: &stageId2Needs,
			},
		})
		So(err, ShouldNotBeNil)
	})
}
