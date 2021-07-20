// +build !unit

package elasticsearch

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/appbaseio/reactivesearch-api/util"

	. "github.com/smartystreets/goconvey/convey"
)

type Nodes struct {
	Total int `json:"total"`
}
type NodeResponse struct {
	Nodes       Nodes  `json:"_nodes"`
	ClusterName string `json:"cluster_name"`
}

func TestElasticsearch(t *testing.T) {
	build := util.BuildArc{}
	util.StartArc(&build)
	build.Start()
	defer build.Close()
	Convey("Misc", t, func() {
		Convey("NodeCount", func() {
			// Set TimeValidity to a positive value
			response, err, _ := util.MakeHttpRequest(http.MethodGet, "/_nodes", nil)
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
