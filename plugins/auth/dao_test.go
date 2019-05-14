package auth

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/appbaseio-confidential/arc/model/acl"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/op"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/olivere/elastic"
)

func newStubClient(url, userIndex, permissionIndex string) (*elasticsearch, error) {
	client, err := elastic.NewSimpleClient(elastic.SetURL(url))
	if err != nil {
		return nil, fmt.Errorf("error while initializing elastic client: %v", err)
	}
	es := &elasticsearch{
		url,
		userIndex, "_doc",
		permissionIndex, "_doc",
		client,
	}
	return es, nil
}

type ServerSetup struct {
	Method, Path, Body, Response string
	HTTPStatus                   int
}

// This function is a modified version of: https://github.com/github/vulcanizer/blob/master/es_test.go
func buildTestServer(t *testing.T, setups []*ServerSetup) *httptest.Server {
	handlerFunc := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBytes, _ := ioutil.ReadAll(r.Body)
		requestBody := string(requestBytes)

		var s string
		matched := false
		for _, setup := range setups {
			s = setup.Method + ":" + setup.Path
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
			/*t.Logf("No requests matched setup. Got method %s, Path %s, body %s\n wanted: method: %s path: %s body: %s\n", r.Method, r.URL.EscapedPath(), requestBody, setup.Method, setup.Path, setup.Body)
			t.Logf("%v Method: %v Path: %v Body: %v\n", matched, r.Method == setup.Method, r.URL.EscapedPath() == setup.Path, requestBody == setup.Body)
			if matched {
				t.Logf("No requests matched setup. Got method %s, Path %s, body %s\n wanted: method: %s path: %s body: %s\n", r.Method, r.URL.EscapedPath(), requestBody, setup.Method, setup.Path, setup.Body)
				t.Logf("%v Method: %v Path: %v Body: %v\n", matched, r.Method == setup.Method, r.URL.EscapedPath() == setup.Path, requestBody == setup.Body)
				break
			}*/
		}

		// TODO: remove before pushing
		/*if r.URL.EscapedPath() != setup.Path {
			t.Fatalf("wanted: %s got: %s\n", setup.Path, r.URL.EscapedPath())
		}*/
		if !matched {
			t.Fatalf("No requests matched setup. Got method %s, Path %s, body %s\n %s\n", r.Method, r.URL.EscapedPath(), requestBody, s)
		}
	})

	return httptest.NewServer(handlerFunc)
}

var getPermissionTest = []struct {
	setup   *ServerSetup
	rawResp []byte
	err     string
}{
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/perm1/_doc/user1",
			Body:     "",
			Response: `{"_index":"perm1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		[]byte(`{"_index":"perm1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"",
	},
}

func TestGetPermission(t *testing.T) {
	for _, tt := range getPermissionTest {
		t.Run("getPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, "test", "perm1")
			_, err := es.getPermission(context.Background(), "user1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(actualPermission, expectedPermission) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", expectedPermission, actualPermission)
			}*/
		})
	}
}

var getUserTest = []struct {
	setup   *ServerSetup
	rawResp []byte
	err     string
}{
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test/_doc/user1",
			Body:     "",
			Response: `{"_index":"user1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		[]byte(`{"_index":"user1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"",
	},
}

func TestGetUser(t *testing.T) {
	for _, tt := range getUserTest {
		t.Run("getPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, "test", "perm1")
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

func newTrue() *bool {
	b := true
	return &b
}

var u = user.User{
	Username:   "user1",
	Password:   "pass1",
	IsAdmin:    newTrue(),
	Categories: []category.Category{category.Docs},
	ACLs:       []acl.ACL{acl.Update},
	Email:      "user1@gmail.com",
	Ops:        []op.Operation{op.Write},
	Indices:    []string{"test"},
	CreatedAt:  "dfds",
}

var putUserTest = []struct {
	setup   *ServerSetup
	rawResp []byte
	err     string
}{
	{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/test/_doc/user1",
			Body:     `{"username":"user1","password":"pass1","is_admin":true,"categories":["docs"],"acls":["update"],"email":"user1@gmail.com","ops":["write"],"indices":["test"],"created_at":"dfds"}`,
			Response: `{"_index":"user1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		[]byte(`{"_index":"user1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"",
	},
}

func TestPutUser(t *testing.T) {
	for _, tt := range putUserTest {
		t.Run("getPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, "test", "perm1")
			_, err := es.putUser(context.Background(), u)

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(actualPermission, expectedPermission) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", expectedPermission, actualPermission)
			}*/
		})
	}
}

var defaultLimits = &permission.Limits{
	IPLimit:       7200,
	DocsLimit:     30,
	SearchLimit:   30,
	IndicesLimit:  30,
	CatLimit:      30,
	ClustersLimit: 30,
	MiscLimit:     30,
}

var perm = permission.Permission{
	Username:    "user1",
	Password:    "pass1",
	Owner:       "owner1",
	Creator:     "creator1",
	Categories:  []category.Category{category.Docs},
	ACLs:        []acl.ACL{acl.Update},
	Ops:         []op.Operation{op.Write},
	Indices:     []string{"test"},
	Sources:     []string{"source"},
	Referers:    []string{"referers"},
	CreatedAt:   "mon",
	TTL:         1,
	Limits:      defaultLimits,
	Description: "permissions payload",
}

var putPermissionTest = []struct {
	setup   *ServerSetup
	rawResp []byte
	err     string
}{
	{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/perm/_doc/user1",
			Body:     `{"username":"user1","password":"pass1","owner":"owner1","creator":"creator1","categories":["docs"],"acls":["update"],"ops":["write"],"indices":["test"],"sources":["source"],"referers":["referers"],"created_at":"mon","ttl":1,"limits":{"ip_limit":7200,"docs_limit":30,"search_limit":30,"indices_limit":30,"cat_limit":30,"clusters_limit":30,"misc_limit":30},"description":"permissions payload"}`,
			Response: `{"_index":"user1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		[]byte(`{"_index":"user1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"",
	},
}

func TestPutPermission(t *testing.T) {
	for _, tt := range putPermissionTest {
		t.Run("getPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, "test", "perm")
			_, err := es.putPermission(context.Background(), perm)

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(actualPermission, expectedPermission) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", expectedPermission, actualPermission)
			}*/
		})
	}
}

var getCredentialTest = []struct {
	setup   *ServerSetup
	rawResp []byte
	err     string
}{
	{
		&ServerSetup{
			Method: "POST",
			Path:   "/test%2Cperm/_search",
			Body:   `{"_source":true,"query":{"bool":{"must":[{"term":{"username.keyword":"user1"}},{"term":{"password.keyword":"pass1"}}]}}}`,
			//Response: `{"took":70,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":0,"max_score":null,"hits":[]}}`,
			Response: `{
				"took": 2,
				"timed_out": false,
				"_shards": {
					"total": 10,
					"successful": 10,
					"skipped": 0,
					"failed": 0
				},
				"hits": {
					"total": 1,
					"max_score": 2.5,
					"hits": [
						{
							"_score": 2.5,
							"_index": "test",
							"_id:": "2"
						}
					]
				}
			}`,
		},
		[]byte(`{"_index":"user1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"",
	},
}

func TestGetCredential(t *testing.T) {
	for _, tt := range getCredentialTest {
		t.Run("getPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, "test", "perm")
			_, err := es.getCredential(context.Background(), "user1", "pass1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(actualPermission, expectedPermission) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", expectedPermission, actualPermission)
			}*/
		})
	}
}
