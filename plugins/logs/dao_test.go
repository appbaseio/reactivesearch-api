package logs

import (
	"context"
	"reflect"
	"testing"

	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/util"
)

var getTotalNodesTest = []struct {
	setup    *ServerSetup
	index    string
	nodes    int
	typeName string
	err      string
}{
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/_nodes/_all/_all%2Cnodes",
			Body:     "",
			Response: `{"cluster_name": "elasticsearch","nodes":{"kxyUith0T3yui4gn6PBbJQ":{"name": "kxyUith","transport_address": "127.0.0.1:9300","host": "127.0.0.1","ip": "127.0.0.1","version": "6.2.4","build_hash": "ccec39f","roles":["master","data","ingest"]}}}`,
		},
		"test1",
		1,
		"_doc",
		"",
	},
	// Test case for getTotalNodes failure with a server config that returns an invalid JSON response
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/_nodes/_all/_all%2Cnodes",
			Body:     "",
			Response: `{"cluster_name": "elasticsearch","nodes:{"kxyUith0T3yui4gn6PBbJQ":{"name": "kxyUith","transport_address": "127.0.0.1:9300","host": "127.0.0.1","ip": "127.0.0.1","version": "6.2.4","build_hash": "ccec39f","roles":["master","data","ingest"]}}}`,
		},
		"test1",
		-1,
		"_doc",
		"invalid character 'k' after object key",
	},
}

func TestGetTotalNodes(t *testing.T) {
	for _, tt := range getTotalNodesTest {
		t.Run("getTotalNodesTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			newStubClient(ts.URL, tt.index)
			nodes, err := util.GetTotalNodes()

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error; wanted: %q got: %q\n", tt.err, err)
			}

			if !reflect.DeepEqual(nodes, tt.nodes) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", tt.nodes, nodes)
			}
		})
	}
}

var indexRecordTest = []struct {
	setup    *ServerSetup
	index    string
	typeName string
}{
	{
		&ServerSetup{
			Method: "POST",
			Path:   "/_bulk",
			Body: `{"index":{"_index":"test1","_type":"_doc"}}
{"indices":["test1"],"category":"docs","request":{"uri":"http://localhost:3000/logs","method":"GET","header":{"User":["user"]},"body":""},"response":{"code":200,"status":"OK","Headers":{"Content-Type":["application/json"]},"body":""},"timestamp":"0001-01-01T00:00:00Z"}
`,
			Response: `{"cluster_name": "elasticsearch","nodes":{"kxyUith0T3yui4gn6PBbJQ":{"name": "kxyUith","transport_address": "127.0.0.1:9300","host": "127.0.0.1","ip": "127.0.0.1","version": "6.2.4","build_hash": "ccec39f","roles":["master","data","ingest"]}}}`,
		},
		"test1",
		"_doc",
	},
	// Test case for indexRecord failure with a server config that returns invalid json response
	{
		&ServerSetup{
			Method: "POST",
			Path:   "/_bulk",
			Body: `{"index":{"_index":"test1","_type":"_doc"}}
{"indices":["test1"],"category":"docs","request":{"uri":"http://localhost:3000/logs","method":"GET","header":{"User":["user"]},"body":""},"response":{"code":200,"status":"OK","Headers":{"Content-Type":["application/json"]},"body":""},"timestamp":"0001-01-01T00:00:00Z"}
`,
			Response: `{cluster_name": "elasticsearch","nodes":{"kxyUith0T3yui4gn6PBbJQ":{"name": "kxyUith","transport_address": "127.0.0.1:9300","host": "127.0.0.1","ip": "127.0.0.1","version": "6.2.4","build_hash": "ccec39f","roles":["master","data","ingest"]}}}`,
		},
		"test1",
		"_doc",
	},
}

// TODO: move such sample structs to a `defaults.go` file
var sampleRecord = record{
	Indices:  []string{"test1"},
	Category: category.Docs,
	Request: struct {
		URI     string              `json:"uri"`
		Method  string              `json:"method"`
		Headers map[string][]string `json:"header"`
		Body    string              `json:"body"`
	}{
		URI:     "http://localhost:3000/logs",
		Method:  "GET",
		Headers: map[string][]string{"User": []string{"user"}},
		Body:    "",
	},
	Response: struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Headers map[string][]string
		Body    string `json:"body"`
	}{
		Code:    200,
		Status:  "OK",
		Headers: map[string][]string{"Content-Type": []string{"application/json"}},
		Body:    "",
	},
}

func TestIndexRecord(t *testing.T) {
	for _, tt := range indexRecordTest {
		t.Run("indexRecord Test", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()

			ctx := context.Background()
			es, _ := newStubClient(ts.URL, tt.index)
			es.indexRecord(ctx, sampleRecord)
		})
	}
}

var getRawLogsTest = []struct {
	setup *ServerSetup
	index string
	from  string
	size  string
	err   string
}{
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_search",
			Body:     `{"from":0,"size":10,"sort":[{"timestamp":{"order":"desc"}}]}`,
			Response: `{"took":90,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3" }}]}}`,
		},
		"test1",
		"0",
		"10",
		"",
	},
	// Test case for getRawLogs failure with an invalid from argument
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_search",
			Body:     `{"from":0,"size":10,"sort":[{"timestamp":{"order":"desc"}}]}`,
			Response: `{"took":90,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3" }}]}}`,
		},
		"test1",
		"#",
		"10",
		"invalid value \"#\" for query param \"from\"",
	},
	// Test case for getRawLogs failure with an invalid size argument
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_search",
			Body:     `{"from":0,"size":10,"sort":[{"timestamp":{"order":"desc"}}]}`,
			Response: `{"took":90,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3" }}]}}`,
		},
		"test1",
		"0",
		"#",
		"invalid value \"#\" for query param \"size\"",
	},
	// Test case for getRawLogs failure with a server config which returns invalid json response
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_search",
			Body:     `{"from":0,"size":10,"sort":[{"timestamp":{"order":"desc"}}]}`,
			Response: `{took":90,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3" }}]}}`,
		},
		"test1",
		"0",
		"10",
		"invalid character 't' looking for beginning of object key string",
	},
}

func TestGetRawLogs(t *testing.T) {
	for _, tt := range getRawLogsTest {
		t.Run("getTotalNodesTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, tt.index)
			_, err := es.getRawLogs(context.Background(), tt.from, tt.size, "index1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v\n", tt.err, err)
			}
		})
	}
}
