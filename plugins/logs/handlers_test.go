package logs

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func setUp(url, logIndex string) {
	os.Setenv("ES_CLUSTER_URL", url)
	os.Setenv(envLogsEsIndex, logIndex)
}

func tearDown(server *httptest.Server) {
	server.Close()
	os.Clearenv()
}

var serverSetup = []*ServerSetup{
	&ServerSetup{
		Method:   "POST",
		Path:     "/.logs/_search",
		Body:     `{"from":0,"size":100,"sort":[{"timestamp":{"order":"desc"}}]}`,
		Response: `{"took":90,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3", "indices" : ["test1", "test2"]}}]}}`,
	},
	&ServerSetup{
		Method:   "POST",
		Path:     "/logs1/_search",
		Body:     `{"from":0,"size":100,"sort":[{"timestamp":{"order":"desc"}}]}`,
		Response: `{"took":90,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0}`,
	},
}

var handlerTests = []struct {
	path         string
	logIndex     string
	expectedResp string
	expectedCode int
}{
	{
		"/_logs",
		defaultLogsEsIndex,
		`{"logs":[{"field1":"value3","indices":["test1","test2"]}],"took":90,"total":1}`,
		200,
	},
	{
		"/_logs",
		"logs1",
		`{"error":{"code":500,"message":"unexpected end of JSON input","status":"Internal Server Error"}}`,
		500,
	},
}

func TestGetLogs(t *testing.T) {
	ts := buildTestServer(t, serverSetup)
	defer tearDown(ts)

	for _, tt := range handlerTests {
		l := Instance()
		setUp(ts.URL, tt.logIndex)
		l.mockInitFunc()

		rw := httptest.NewRecorder()

		handler := http.HandlerFunc(l.getLogs())

		req, _ := http.NewRequest("GET", fmt.Sprintf("%s%s", ts.URL, tt.path), nil)
		req.Header.Set("Content-Type", "application/json")

		handler.ServeHTTP(rw, req)

		resp := rw.Result()
		body, _ := ioutil.ReadAll(resp.Body)

		if status := resp.StatusCode; status != tt.expectedCode {
			t.Errorf("handler returned invalid status code: got %v want %v", status, tt.expectedCode)
		}

		actual := strings.TrimSpace(string(body))
		if !reflect.DeepEqual(actual, tt.expectedResp) {
			t.Errorf("handler returned invalid response; got: %v want: %v", actual, tt.expectedResp)
		}
	}
}
