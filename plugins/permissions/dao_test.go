package permissions

import (
	"context"
	"reflect"
	"testing"

	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/util"
)

// We don't use mocks for this DAO since none of the methods are dependent on each other
// and all of them are isolated so they can be tested without passing mock interfaces

var getTotalNodesTest = []struct {
	setup *ServerSetup
	index string
	nodes int
	err   string
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
		"",
	},
	// use an invalid json response from the test server to make the method under test return an error
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/_nodes/_all/_all%2Cnodes",
			Body:     "",
			Response: `{cluster_name": "elasticsearch","nodes":{"kxyUith0T3yui4gn6PBbJQ":{"name": "kxyUith","transport_address": "127.0.0.1:9300","host": "127.0.0.1","ip": "127.0.0.1","version": "6.2.4","build_hash": "ccec39f","roles":["master","data","ingest"]}}}`,
		},
		"test1",
		-1,
		"invalid character 'c' looking for beginning of object key string",
	},
}

func TestGetTotalNodes(t *testing.T) {
	for _, tt := range getTotalNodesTest {
		t.Run("getTotalNodesTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
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

var getPermissionTest = []struct {
	setup   *ServerSetup
	index   string
	rawResp []byte
	err     string
}{
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		"test1",
		[]byte(`{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"",
	},
	// use an invalid json response from the test server to make the method under test return an error
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`,
		},
		"test1",
		[]byte(`{"_index":"test1","_type":"doc","_id":"user1","_version":1,"found":true,"_source":{"first_name":"John","last_name":"Smith","age":25}}`),
		"invalid character '_' looking for beginning of object key string",
	},
	// TODO add test case for JSON Unmarshall error
}

func TestGetPermission(t *testing.T) {
	for _, tt := range getPermissionTest {
		t.Run("getTotalNodesTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, tt.index, "mapping")
			_, err := es.getPermission(context.Background(), "user1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error wanted: %v got: %v\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(actualPermission, expectedPermission) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", expectedPermission, actualPermission)
			}*/
		})
	}
}

var deletePermissionTest = []struct {
	setup     *ServerSetup
	response  bool
	indexName string
	err       string
}{
	{
		&ServerSetup{
			Method:   "DELETE",
			Path:     "/test1/_doc/user1",
			Body:     "",
			Response: `{"_index":"test1","_type":"_doc","_id":"user1","_version":2,"result":"deleted","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":1,"_primary_term":1}`,
		},
		true,
		"test1",
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
		"missing required fields: [Index]",
	},
}

func TestDeletePermission(t *testing.T) {
	for _, tt := range deletePermissionTest {
		t.Run("getUserTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, tt.indexName, "mapping")
			response, err := es.deletePermission(context.Background(), "user1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("deletePermission should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			if !reflect.DeepEqual(response, tt.response) {
				t.Fatalf("Wrong response returned expected: %v got: %v\n", tt.response, response)
			}
		})
	}
}

var patchPermissionTest = []struct {
	setup     *ServerSetup
	response  []byte
	indexName string
	patchMap  map[string]interface{}
	err       string
}{
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_update/user1",
			Body:     `{"doc":{"acls":["docs"],"categories":["search"],"indices":["*"]}}`,
			Response: `{"_index":"test","_type":"_doc","_id":"1","_version":11,"result":"updated","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":13,"_primary_term":6,"get":{"found":true,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}}`,
		},
		//[]byte(`{"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}`),
		[]byte(`{"_source":{"counter":0,"tags":[]}}`),
		"test1",
		map[string]interface{}{"indices": []string{"*"}, "acls": []string{"docs"}, "categories": []string{"search"}},
		"",
	},
	// use an invalid json response from the test server to make the method under test return an error
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_update/user1",
			Body:     `{"doc":{"acls":["docs"],"categories":["search"],"indices":["*"]}}`,
			Response: `{_index":"test","_type":"_doc","_id":"1","_version":11,"result":"updated","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":13,"_primary_term":6,"get":{"found":true,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}}`,
		},
		//[]byte(`{"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}`),
		[]byte(`{"_source":{"counter":0,"tags":[]}}`),
		"test1",
		map[string]interface{}{"indices": []string{"*"}, "acls": []string{"docs"}, "categories": []string{"search"}},
		"invalid character '_' looking for beginning of object key string",
	},
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_update/user1",
			Body:     `{"doc":{"acls":["docs"],"categories":["search"],"indices":["*"]}}`,
			Response: `{"_index":"test","_type":"_doc","_id":"1","_version":11,"result":"updated","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":13,"_primary_term":6,"get":{"found":true,"_source":{counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}}`,
		},
		//[]byte(`{"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}`),
		[]byte(`{"_source":{"counter":0,"tags":[]}}`),
		"test1",
		map[string]interface{}{"indices": []string{"*"}, "acls": []string{"docs"}, "categories": []string{"search"}},
		"invalid character 'c' looking for beginning of object key string",
	},
}

func TestPatchPermission(t *testing.T) {
	for _, tt := range patchPermissionTest {
		t.Run("patchPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, tt.indexName, "mapping")
			_, err := es.patchPermission(context.Background(), "user1", tt.patchMap)

			if !compareErrs(tt.err, err) {
				t.Fatalf("deletePermission should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(response, tt.response) {
				t.Fatalf("Wrong response returned expected: %v got: %v\n", tt.response, response)
			}*/
		})
	}
}

var defaultLimits = &permission.Limits{
	IPLimit:          7200,
	DocsLimit:        30,
	SearchLimit:      30,
	IndicesLimit:     30,
	CatLimit:         30,
	ClustersLimit:    30,
	MiscLimit:        30,
	UserLimit:        30,
	PermissionLimit:  30,
	AnalyticsLimit:   30,
	RulesLimit:       30,
	TemplatesLimit:   30,
	SuggestionsLimit: 30,
	StreamsLimit:     30,
}

var p = permission.Permission{
	Username:   "user1",
	Password:   "a12sfa1",
	Owner:      "creator1",
	Creator:    "creator1",
	Categories: []category.Category{category.Docs},
	Ops:        []op.Operation{op.Write},
	Indices:    []string{"*"},
	Sources:    []string{"0.0.0.0/0"},
	Referers:   []string{"*"},
	CreatedAt:  "2019-03-25T10:24:28+05:30",
	TTL:        -1,
	Limits:     defaultLimits,
}

var postPermissionTest = []struct {
	setup *ServerSetup
	//response  []byte
	indexName string
	payload   permission.Permission
	err       string
}{
	{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/test1/_doc/user1",
			Body:     `{"username":"user1","password":"a12sfa1","owner":"creator1","creator":"creator1","categories":["docs"],"acls":null,"ops":["write"],"indices":["*"],"sources":["0.0.0.0/0"],"referers":["*"],"created_at":"2019-03-25T10:24:28+05:30","ttl":-1,"limits":{"ip_limit":7200,"docs_limit":30,"search_limit":30,"indices_limit":30,"cat_limit":30,"clusters_limit":30,"misc_limit":30},"description":""}`,
			Response: `{"_index":"test","_type":"_doc","_id":"1","_version":11,"result":"updated","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":13,"_primary_term":6,"get":{"found":true,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}}`,
		},
		//[]byte(`{"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}`),
		//[]byte(`{"_source":{"counter":0,"tags":[]}}`),
		"test1",
		p,
		"",
	},
	// use an invalid json response from the test server to make the method under test return an error
	{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/test1/_doc/user1",
			Body:     `{"username":"user1","password":"a12sfa1","owner":"creator1","creator":"creator1","categories":["docs"],"acls":null,"ops":["write"],"indices":["*"],"sources":["0.0.0.0/0"],"referers":["*"],"created_at":"2019-03-25T10:24:28+05:30","ttl":-1,"limits":{"ip_limit":7200,"docs_limit":30,"search_limit":30,"indices_limit":30,"cat_limit":30,"clusters_limit":30,"misc_limit":30},"description":""}`,
			Response: `{_index":"test","_type":"_doc","_id":"1","_version":11,"result":"updated","_shards":{"total":2,"successful":1,"failed":0},"_seq_no":13,"_primary_term":6,"get":{"found":true,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}}}`,
		},
		"test1",
		p,
		"invalid character '_' looking for beginning of object key string",
	},
}

func TestPostPermission(t *testing.T) {
	for _, tt := range postPermissionTest {
		t.Run("patchPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, tt.indexName, "mapping")
			_, err := es.postPermission(context.Background(), tt.payload)

			if !compareErrs(tt.err, err) {
				t.Fatalf("deletePermission should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(response, tt.response) {
				t.Fatalf("Wrong response returned expected: %v got: %v\n", tt.response, response)
			}*/
		})
	}
}

var getRawOwnerPermissionsTest = []struct {
	setup     *ServerSetup
	indexName string
	err       string
}{
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_doc/_search",
			Body:     `{"query":{"term":{"owner.keyword":"owner1"}}}`,
			Response: `{"took":107,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3" }}]}}`,
		},
		"test1",
		"",
	},
	// use an invalid json response from the test server to make the method under test return an error
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/test1/_doc/_search",
			Body:     `{"query":{"term":{"owner.keyword":"owner1"}}}`,
			Response: `{took":107,"timed_out":false,"_shards":{"total":5,"successful":5,"skipped":0,"failed":0},"hits":{"total":2,"max_score":1.0,"hits":[{"_index":"test","_type":"_doc","_id":"1","_score":1.0,"_source":{"counter":5,"tags":["red","blue","blue","blue","blue","blue"]}},{"_index":"test","_type":"_doc","_id":"3","_score":1.0,"_source":{ "field1" : "value3" }}]}}`,
		},
		"test1",
		"invalid character 't' looking for beginning of object key string",
	},
}

func TestGetRawOwnerPermissions(t *testing.T) {
	for _, tt := range getRawOwnerPermissionsTest {
		t.Run("patchPermissionTest", func(t *testing.T) {
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newStubClient(ts.URL, tt.indexName, "mapping")
			_, err := es.getRawOwnerPermissions(context.Background(), "owner1")

			if !compareErrs(tt.err, err) {
				t.Fatalf("deletePermission should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			/*if !reflect.DeepEqual(response, tt.response) {
				t.Fatalf("Wrong response returned expected: %v got: %v\n", tt.response, response)
			}*/
		})
	}
}
