package util


// TODO: Need to fix the path relates issues
// import (
// 	"encoding/json"
// 	"net/http"
// 	"testing"

// 	. "github.com/smartystreets/goconvey/convey"
// )

// func TestUtil(t *testing.T) {
// 	build := BuildArc{}
// 	StartArc(&build)
// 	build.Start()
// 	defer build.Close()
// 	Convey("Misc", t, func() {
// 		Convey("NodeCount", func() {
// 			// Set TimeValidity to a positive value
// 			response, err, _ := MakeHttpRequest(http.MethodGet, "/_nodes", nil)
// 			if err != nil {
// 				t.Fatalf("Unable to fetch node count: %v", err)
// 			}
// 			parsedResponse, _ := response.(map[string]interface{})
// 			var nodesResponse NodeResponse
// 			marshalled, _ := json.Marshal(parsedResponse)
// 			json.Unmarshal(marshalled, &nodesResponse)
// 			if nodesResponse.Nodes.Total <= 0 {
// 				t.Fatalf("Node count must have a non-zero value, found %v nodes", nodesResponse.Nodes.Total)
// 			}
// 		})
// 	})
// }
