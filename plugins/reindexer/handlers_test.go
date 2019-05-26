package reindexer

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/gorilla/mux"
)

func setUp(url string) {
	os.Setenv(envEsURL, url)
}

func tearDown(server *httptest.Server) {
	server.Close()
	os.Clearenv()
}

var serverSetup = []*ServerSetup{
	&ServerSetup{
		Method:   "POST",
		Path:     "/_reindex",
		Body:     `{"dest":{"index":"test_reindexed_1"},"source":{"_source":{"excludes":["details"],"includes":["username"]},"index":"test"}}`,
		Response: `{"took":87,"timed_out":false,"total":2,"updated":2,"created":0,"deleted":0,"batches":1,"version_conflicts":0,"noops":0,"retries":{"bulk":0,"search":0},"throttled_millis":0,"requests_per_second":-1.0,"throttled_until_millis":0,"failures":[]}`,
	},
	&ServerSetup{
		Method:   "GET",
		Path:     "/_tasks/",
		Body:     "",
		Response: `{"took":87,"timed_out":false,"total":2,"updated":2,"created":0,"deleted":0,"batches":1,"version_conflicts":0,"noops":0,"retries":{"bulk":0,"search":0},"throttled_millis":0,"requests_per_second":-1.0,"throttled_until_millis":0,"failures":[]}`,
	},
	&ServerSetup{
		Method:   "GET",
		Path:     "/test/_settings",
		Body:     "",
		Response: `{"test":{"settings":{"index":{"creation_date":"1552665579942","number_of_shards":"5","number_of_replicas":"1","uuid":"hqhO4oiCReawwtOqFHaVLA","version":{"created":"6020499"},"provided_name":"test"}}}}`,
	},
	&ServerSetup{
		Method:   "GET",
		Path:     "/test/_mapping/_all",
		Body:     "",
		Response: `{"test":{"mappings":{"_all":{"properties":{"counter":{"type":"long"},"field1":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}},"field2":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}},"tags":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}}}}}}}`,
	},
	&ServerSetup{
		Method:   "GET",
		Path:     "/test/_alias",
		Body:     "",
		Response: `{"test":{"aliases":{"alias1":{},"alias2":{}}}}`,
	},
	&ServerSetup{
		Method:   "POST",
		Path:     "/_aliases",
		Body:     `{"actions":[{"add":{"alias":"test","index":"test_reindexed_1"}}]}`,
		Response: `{"acknowledged": true, "shards_acknowledged": true, "index": "test"}`,
	},
	&ServerSetup{
		Method:   "DELETE",
		Path:     "/test",
		Body:     "",
		Response: `{"acknowledged": true}`,
	},
	&ServerSetup{
		Method:   "PUT",
		Path:     "/test_reindexed_1",
		Body:     `{"mappings":{"_all":{"properties":{"counter":{"type":"long"},"field1":{"fields":{"keyword":{"ignore_above":256,"type":"keyword"}},"type":"text"},"field2":{"fields":{"keyword":{"ignore_above":256,"type":"keyword"}},"type":"text"},"tags":{"fields":{"keyword":{"ignore_above":256,"type":"keyword"}},"type":"text"}}}},"settings":{"index":{},"number_of_replicas":"1","number_of_shards":"5"}}`,
		Response: `{"acknowledged": true, "shards_acknowledged": true, "index": "test"}`,
	},
	&ServerSetup{
		Method: "GET",
		Path:   "/_cat/aliases",
		Body:   "",
		Response: `[{
			"alias": "alias1",
			"index": "test1",
			"filter": "-",
			"routing.index": "-",
			"routing.search": "-"
		},
		{
			"alias": "alias2",
			"index": "test2",
			"filter": "-",
			"routing.index": "-",
			"routing.search": "-"
		}]`,
	},
}

var handlerTests = []struct {
	path         string
	muxVar       string
	reqBody      []byte
	expectedResp string
	expectedCode int
}{
	{
		"/_reindex/test",
		"test",
		[]byte(`{"include_fields": ["username"],"exclude_fields": ["details"]}`),
		`{"took":87,"timed_out":false,"total":2,"updated":2,"deleted":0,"batches":1,"version_conflicts":0,"noops":0,"retries":{"bulk":0,"search":0},"throttled":"","throttled_millis":0,"requests_per_second":-1,"throttled_until":"","throttled_until_millis":0,"failures":[]}`,
		200,
	},
	/*{
		"/_reindex/test",
		"",
		[]byte(`{"include_fields": ["username"],"exclude_fields": ["details"]}`),
		`{"error":{"code":500,"message":"Route inconsistency, expecting var {index}","status":"Internal Server Error"}}`,
		500,
	},
	{
		"/_reindex/test",
		"test",
		[]byte(`{"include_fields": ["username"],"exclude_fields: ["details"]}`),
		`{"error":{"code":400,"message":"Can't parse request body","status":"Bad Request"}}`,
		400,
	},*/
}

func TestReindexHandler(t *testing.T) {
	ts := buildTestServer(t, serverSetup)
	defer tearDown(ts)

	for _, tt := range handlerTests {
		rx := Instance()
		setUp(ts.URL)
		rx.mockInitFunc()

		rw := httptest.NewRecorder()

		handler := http.HandlerFunc(rx.reindex())

		req, _ := http.NewRequest("POST", fmt.Sprintf("%s%s", ts.URL, tt.path), bytes.NewBuffer(tt.reqBody))
		if tt.muxVar != "" {
			req = mux.SetURLVars(req, map[string]string{"index": "test"})
		}
		req.Header.Set("Content-Type", "application/json")

		handler.ServeHTTP(rw, req)

		resp := rw.Result()
		body, _ := ioutil.ReadAll(resp.Body)

		if status := resp.StatusCode; status != tt.expectedCode {
			t.Errorf("handler returned invalid status code: got %v want %v", status, tt.expectedCode)
		}

		actual := string(body)
		if !reflect.DeepEqual(actual, tt.expectedResp) {
			t.Errorf("handler returned invalid response; got: %v want: %v", actual, tt.expectedResp)
		}
	}
}
