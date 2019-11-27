package users

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/appbaseio/arc/util"
)

func newStubClient(indexName string) (*elasticsearch, error) {
	es := &elasticsearch{indexName}
	return es, nil
}

type ServerSetup struct {
	Method, Path, Body, Response string
	HTTPStatus                   int
}

// This function is a modified version of: https://github.com/github/vulcanizer/blob/master/es_test.go
func buildTestServer(t *testing.T, setup *ServerSetup) *httptest.Server {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, _ := ioutil.ReadAll(r.Body)
		requestBody := string(requestBytes)

		matched := false
		if r.Method == setup.Method && r.URL.EscapedPath() == setup.Path && requestBody == setup.Body {
			matched = true
			if setup.HTTPStatus == 0 {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(setup.HTTPStatus)
			}
			_, err := w.Write([]byte(setup.Response))
			if err != nil {
				t.Fatalf("Unable to write test server response: %v", err)
			}
		}

		// TODO: remove before pushing
		/*if !reflect.DeepEqual(r.URL.EscapedPath(), setup.Path) {
			t.Fatalf("wanted: %s got: %s\n", setup.Path, r.URL.EscapedPath())
		}*/
		if !matched {
			t.Fatalf("No requests matched setup. Got method %s, Path %s, body %s\n", r.Method, r.URL.EscapedPath(), requestBody)
		}
	})

	return httptest.NewServer(handlerFunc)
}

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
}

func TestGetTotalNodes(t *testing.T) {
	for _, tt := range getTotalNodesTest {
		t.Run("getTotalNodesTest", func(t *testing.T) {
			ts := buildTestServer(t, tt.setup)
			defer ts.Close()
			nodes, err := util.GetTotalNodes()

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			if !reflect.DeepEqual(nodes, tt.nodes) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", tt.nodes, nodes)
			}
		})
	}
}

var getUserTest = []struct {
	setup    *ServerSetup
	rawResp  []byte
	typeName string
	err      string
}{
	// valid request response
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		[]byte(`{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"_doc",
		"",
	},
	// missing typeName
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		nil,
		"",
		"missing required fields: [Type]",
	},
	// bad json response
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}`,
		},
		nil,
		"_doc",
		"unexpected end of JSON input",
	},
}

func TestGetUser(t *testing.T) {
	for _, tt := range getUserTest {
		t.Run("getUserTest", func(t *testing.T) {
			ts := buildTestServer(t, tt.setup)
			defer ts.Close()
			es, _ := newStubClient("test1")
			_, err := es.getUser(context.Background(), "user1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(actualPermission, expectedPermission) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", expectedPermission, actualPermission)
			}*/
		})
	}
}

var deleteUserTest = []struct {
	setup    *ServerSetup
	response bool
	typeName string
	err      string
}{
	{
		&ServerSetup{
			Method:   "DELETE",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"_doc","_id":"user1","_version":2,"result":"deleted","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":1,"_primary_term":1}`,
		},
		true,
		"_doc",
		"",
	},
	{
		&ServerSetup{
			Method:   "DELETE",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"_doc","_id":"user1","_version":2,"result":"deleted","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":1,"_primary_term":1}`,
		},
		false,
		"",
		"missing required fields: [Type]",
	},
}

func TestDeleteUser(t *testing.T) {
	for _, tt := range deleteUserTest {
		t.Run("getUserTest", func(t *testing.T) {
			ts := buildTestServer(t, tt.setup)
			defer ts.Close()
			es, _ := newStubClient("test1")
			response, err := es.deleteUser(context.Background(), "user1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("deleteUser should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			if !reflect.DeepEqual(response, tt.response) {
				t.Fatalf("Wrong response returned expected: %v got: %v\n", tt.response, response)
			}
		})
	}
}
