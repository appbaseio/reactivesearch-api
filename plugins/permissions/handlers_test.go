package permissions

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/appbaseio/arc/model/user"

	"github.com/gorilla/mux"
)

func setUp(url string) {
	os.Setenv(envEsURL, url)
}

func tearDown(server *httptest.Server) {
	server.Close()
	os.Clearenv()
}

var getPermissionHandlerTests = []struct {
	path         string
	reqBody      []byte
	muxVar       string
	expectedResp string
	expectedCode int
}{
	{
		"/_permission/2fa2854c7e08",
		[]byte(``),
		"user1",
		`{"first_name":"John","last_name":"Smith","age":25}`,
		200,
	},
	{
		"/_permission/2fa2854c7e08",
		[]byte(``),
		"",
		`{"error":{"code":404,"message":"permission with \"username\"=\"\" not found","status":"Not Found"}}`,
		404,
	},
}

func TestGetPermissionHandler(t *testing.T) {
	serverSetup := []*ServerSetup{
		&ServerSetup{
			Method:   "GET",
			Path:     "/.permissions/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
	}

	ts := buildTestServer(t, serverSetup)
	defer tearDown(ts)

	for _, tt := range getPermissionHandlerTests {
		p := Instance()
		setUp(ts.URL)
		p.mockInitFunc()

		rw := httptest.NewRecorder()

		handler := http.HandlerFunc(p.getPermission())

		req, _ := http.NewRequest("GET", fmt.Sprintf("%s%s", ts.URL, tt.path), bytes.NewBuffer(tt.reqBody))
		if tt.muxVar != "" {
			req = mux.SetURLVars(req, map[string]string{"username": tt.muxVar})
		}
		req.Header.Set("Content-Type", "application/json")

		handler.ServeHTTP(rw, req)

		resp := rw.Result()
		body, _ := ioutil.ReadAll(resp.Body)

		if status := resp.StatusCode; status != tt.expectedCode {
			t.Errorf("handler returned invalid status code: got %v want %v", status, tt.expectedCode)
		}

		actual := strings.TrimSpace(string(body))
		if !reflect.DeepEqual(actual, tt.expectedResp) {
			t.Errorf("handler returned invalid response; got: %q want: %q", actual, tt.expectedResp)
		}
	}
}

var sampleUser = &user.User{
	Username: "user1",
}

var postPermissionHandlerTests = []struct {
	path         string
	reqBody      []byte
	ctx          context.Context
	expectedResp string
	expectedCode int
}{
	/*{
		"/_permission/",
		[]byte(`{"user_id":"foo","acls":[\"docs\"],"categories":["search"]}`),
		context.WithValue(context.Background(), "user", sampleUser),
		`{"took":87,"timed_out":false,"total":2,"updated":2,"deleted":0,"batches":1,"version_conflicts":0,"noops":0,"retries":{"bulk":0,"search":0},"throttled":"","throttled_millis":0,"requests_per_second":-1,"throttled_until":"","throttled_until_millis":0,"failures":[]}`,
		200,
	},*/
	{
		"/_permission/",
		[]byte(`{"user_id":"foo","acls":[\"docs\"],"categories":["search"]}`),
		context.Background(),
		`{"error":{"code":500,"message":"\"*user.User\" not found in request context","status":"Internal Server Error"}}`,
		500,
	},
}

func TestPostPermissionHandler(t *testing.T) {
	serverSetup := []*ServerSetup{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/test1/_doc/user1",
			Body:     `{"username":"user1","password":"a12sfa1","owner":"creator1","creator":"creator1","categories":["docs"],"acls":null,"ops":["write"],"indices":["*"],"sources":["0.0.0.0/0"],"referers":["*"],"created_at":"2019-03-25T10:24:28+05:30","ttl":-1,"limits":{"ip_limit":7200,"docs_limit":30,"search_limit":30,"indices_limit":30,"cat_limit":30,"clusters_limit":30,"misc_limit":30},"description":""}`,
			Response: `{"_index":"test","_type":"_doc","_id":"1","_version":11,"result":"updated","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":13,"_primary_term":6,"get":{"found":true,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}}`,
		},
	}

	ts := buildTestServer(t, serverSetup)
	defer tearDown(ts)

	for _, tt := range postPermissionHandlerTests {
		p := Instance()
		setUp(ts.URL)
		p.mockInitFunc()

		rw := httptest.NewRecorder()

		handler := http.HandlerFunc(p.postPermission())

		req, _ := http.NewRequest("POST", fmt.Sprintf("%s%s", ts.URL, tt.path), bytes.NewBuffer(tt.reqBody))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(tt.ctx)
		log.Printf("%v", req.Context().Value("user"))

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

var patchPermissionHandlerTests = []struct {
	path         string
	reqBody      []byte
	expectedResp string
	expectedCode int
}{
	// Bad request; JSON unmarshal error
	{
		"/_permission/2fa2854c7e08",
		[]byte(`{"user_id":"foo","acls":[\"docs\"],"categories":["search"]}`),
		`{"error":{"code":400,"message":"can't parse request body","status":"Bad Request"}}`,
		400,
	},
	// Bad request GetPatch error
	{
		"/_permission/2fa2854c7e08",
		[]byte(`{"username":"user1","password":"a12sfa1","owner":"creator1","creator":"creator1","categories":["docs"],"acls":null,"ops":["write"],"indices":["*"],"sources":["0.0.0.0/0"],"referers":["*"],"created_at":"2019-03-25T10:24:28+05:30","ttl":-1,"limits":{"ip_limit":7200,"docs_limit":30,"search_limit":30,"indices_limit":30,"cat_limit":30,"clusters_limit":30,"misc_limit":30},"description":""}`),
		`{"error":{"code":400,"message":"cannot patch field \"username\" in permission","status":"Bad Request"}}`,
		400,
	},
}

func TestPatchPermissionHandler(t *testing.T) {
	serverSetup := []*ServerSetup{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_doc/user1/_update",
			Body:     `{"doc":{"acls":["docs"],"categories":["search"],"indices":["*"]}}`,
			Response: `{"_index":"test","_type":"_doc","_id":"1","_version":11,"result":"updated","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":13,"_primary_term":6,"get":{"found":true,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}}`,
		},
	}

	ts := buildTestServer(t, serverSetup)
	defer tearDown(ts)

	for _, tt := range patchPermissionHandlerTests {
		p := Instance()
		setUp(ts.URL)
		p.mockInitFunc()

		rw := httptest.NewRecorder()

		handler := http.HandlerFunc(p.patchPermission())

		req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s%s", ts.URL, tt.path), bytes.NewBuffer(tt.reqBody))
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

var deletePermissionHandlerTests = []struct {
	path         string
	reqBody      []byte
	expectedResp string
	expectedCode int
}{
	{
		"/_permission/2fa2854c7e08",
		[]byte(``),
		`{"error":{"code":404,"message":"permission with \"username\"=\"\" not found","status":"Not Found"}}`,
		404,
	},
}

func TestDeletePermissionHandler(t *testing.T) {
	serverSetup := []*ServerSetup{
		&ServerSetup{
			Method:   "DELETE",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"_doc","_id":"user1","_version":2,"result":"deleted","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":1,"_primary_term":1}`,
		},
	}

	ts := buildTestServer(t, serverSetup)
	defer tearDown(ts)

	for _, tt := range deletePermissionHandlerTests {
		p := Instance()
		setUp(ts.URL)
		p.mockInitFunc()

		rw := httptest.NewRecorder()

		handler := http.HandlerFunc(p.deletePermission())

		req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s%s", ts.URL, tt.path), bytes.NewBuffer(tt.reqBody))
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

var getUserPermissionHandlerTests = []struct {
	path         string
	reqBody      []byte
	expectedResp string
	expectedCode int
}{
	{
		"/_permissions",
		[]byte(``),
		`[{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]},{"field1":"value3"}]`,
		200,
	},
}

func TestGetUserPermissionHandler(t *testing.T) {
	serverSetup := []*ServerSetup{
		&ServerSetup{
			Method:   "POST",
			Path:     "/.permissions/_doc/_search",
			Body:     `{"query":{"term":{"owner.keyword":""}}}`,
			Response: `{"took":107,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3" }}]}}`,
		},
	}

	ts := buildTestServer(t, serverSetup)
	defer tearDown(ts)

	for _, tt := range getUserPermissionHandlerTests {
		p := Instance()
		setUp(ts.URL)
		p.mockInitFunc()

		rw := httptest.NewRecorder()

		handler := http.HandlerFunc(p.getUserPermissions())

		req, _ := http.NewRequest("GET", fmt.Sprintf("%s%s", ts.URL, tt.path), bytes.NewBuffer(tt.reqBody))
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
