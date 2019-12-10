package util

import (
	"encoding/json"
	"net/http"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type Nodes struct {
	Total int `json:"total"`
}
type NodeResponse struct {
	Nodes       Nodes  `json:"_nodes"`
	ClusterName string `json:"cluster_name"`
}

func TestBilling(t *testing.T) {
	Convey("Billing", t, func() {
		Convey("Set Tier", func() {
			var plan = Sandbox
			SetTier(&plan)
			So(GetTier().String(), ShouldEqual, Sandbox.String())
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
		Convey("Validate TimeValidity: Positive Value", func() {
			// Set TimeValidity to a positive value
			var timeValidityMock = 1200000
			SetTimeValidity(int64(timeValidityMock))
			So(true, ShouldEqual, validateTimeValidity())
		})
		Convey("Validate TimeValidity: Negative value greater than 24 hours", func() {
			// Set TimeValidity to a positive value
			var timeValidityMock = -(3600*24 + 10)
			SetTimeValidity(int64(timeValidityMock))
			So(false, ShouldEqual, validateTimeValidity())
		})
		Convey("Validate TimeValidity: Negative value less than 24 hours", func() {
			// Set TimeValidity to a positive value
			var timeValidityMock = -(3600*24 - 10)
			SetTimeValidity(int64(timeValidityMock))
			So(true, ShouldEqual, validateTimeValidity())
		})
		Convey("NodeCount", func() {
			// Set TimeValidity to a positive value
			response, err := MakeHttpRequest(http.MethodGet, "/_nodes", nil)
			if err != nil {
				t.Fatalf("Unable to fetch node count: %v", err)
			}
			parsedResponse, _ := response.(map[string]interface{})
			var nodesResponse NodeResponse
			marshalled, _ := json.Marshal(parsedResponse)
			json.Unmarshal(marshalled, &nodesResponse)
			if nodesResponse.Nodes.Total <= 0 {
				t.Fatalf("Node count must have a non-zero value, found %v nodes", nodesResponse.Nodes.Total)
			}
		})
	})
}
