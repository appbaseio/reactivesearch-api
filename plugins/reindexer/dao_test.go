package reindexer

import (
	"context"
	"reflect"
	"testing"
)

var aliasesOfTests = []struct {
	setup   *ServerSetup
	index   string
	aliases []string
	err     string
}{
	{
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
		"test1",
		[]string{"alias1"},
		"",
	},
	{
		&ServerSetup{
			Method: "GET",
			Path:   "/_cat/aliases",
			Body:   "",
			Response: `{
				"alias": "alias1",
				"index": "test1",
				"filter": "-",
				"routing.index": "-",
				"routing.search": "-"
			}`,
		},
		"test1",
		nil,
		"json: cannot unmarshal object into Go value of type elastic.CatAliasesResponse",
	},
}

func TestAliasesOf(t *testing.T) {
	for _, tt := range aliasesOfTests {
		t.Run("Should successfully create index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newTestClient(ts.URL)
			aliases, err := es.aliasesOf(ctx, tt.index)

			if !compareErrs(tt.err, err) {
				t.Fatalf("Cat aliases should have failed with error: %v got: %v instead\n", tt.err, err)
			}

			if !reflect.DeepEqual(aliases, tt.aliases) {
				t.Fatalf("Wrong aliases returned expected: %v got: %v\n", tt.aliases, aliases)
			}
		})
	}
}

var createIndexTests = []struct {
	setup *ServerSetup
	index string
	err   string
}{
	{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/test",
			Body:     `null`,
			Response: `{"acknowledged": true, "shards_acknowledged": true, "index": "test"}`,
		},
		"test",
		"",
	},
	{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/test",
			Body:     `null`,
			Response: `{"acknowledged": false, "shards_acknowledged": false, "index": "test"}`,
		},
		"test",
		"failed to create index named \"test\", acknowledged=false",
	},
	{
		&ServerSetup{
			Method:   "PUT",
			Path:     "/",
			Body:     `null`,
			Response: "",
		},
		"",
		"missing index name",
	},
}

func TestCreateIndex(t *testing.T) {
	for _, tt := range createIndexTests {
		t.Run("Should successfully create index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newTestClient(ts.URL)
			err := es.createIndex(ctx, tt.index, nil)
			if !compareErrs(tt.err, err) {
				t.Fatalf("Index creation should have failed with error: %v got: %v instead\n", tt.err, err)
			}
		})
	}
}

var deleteIndexTests = []struct {
	setup *ServerSetup
	index string
	err   string
}{
	{
		&ServerSetup{
			Method:   "DELETE",
			Path:     "/test",
			Body:     "",
			Response: `{"acknowledged": true}`,
		},
		"test",
		"",
	},
	{
		&ServerSetup{
			Method:   "DELETE",
			Path:     "/test",
			Body:     "",
			Response: `{"acknowledged": false}`,
		},
		"test",
		"error deleting index \"test\", acknowledged=false",
	},
	// TODO: Add test for unexpected error
}

func TestDeleteIndex(t *testing.T) {
	for _, tt := range deleteIndexTests {
		t.Run("Should successfully delete index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newTestClient(ts.URL)
			err := es.deleteIndex(ctx, tt.index)
			if !compareErrs(tt.err, err) {
				t.Fatalf("Index deletion should have failed with error: %v got: %v instead\n", tt.err, err)
			}
		})
	}
}

var setAliasTests = []struct {
	setup   *ServerSetup
	index   string
	aliases []string
	err     string
}{
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/_aliases",
			Body:     `{"actions":[{"add":{"alias":"alias1","index":"test"}},{"add":{"alias":"alias2","index":"test"}}]}`,
			Response: `{"acknowledged": true, "shards_acknowledged": true, "index": "test"}`,
		},
		"test",
		[]string{"alias1", "alias2"},
		"",
	},
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/_aliases",
			Body:     `{"actions":[{"add":{"alias":"alias1","index":"test"}},{"add":{"alias":"alias2","index":"test"}}]}`,
			Response: `{"acknowledged": false, "shards_acknowledged": false, "index": "test"}`,
		},
		"test",
		[]string{"alias1", "alias2"},
		"unable to set aliases \"[alias1 alias2]\" for index \"test\"",
	},
	{
		&ServerSetup{
			Method:   "POST",
			Path:     "/_aliases",
			Body:     `{"actions":[{"add":{"alias":"alias1","index":"test"}},{"add":{"alias":"alias2","index":"test"}}]}`,
			Response: "",
		},
		"",
		[]string{"alias1", "alias2"},
		"missing required fields: [Index]",
	},
}

func TestSetAlias(t *testing.T) {
	for _, tt := range setAliasTests {
		t.Run("Should successfully delete index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newTestClient(ts.URL)
			err := es.setAlias(ctx, tt.index, tt.aliases[0], tt.aliases[1])
			if !compareErrs(tt.err, err) {
				t.Fatalf("Index creation should have failed with error: %v got: %v instead\n", tt.err, err)
			}
		})
	}
}

var getIndicesByAliasTests = []struct {
	setup *ServerSetup
	alias string
	resp  []string
	err   string
}{
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test/_alias",
			Body:     "",
			Response: `{"test":{"aliases":{"alias1":{},"alias2":{}}}}`,
		},
		"test",
		nil,
		"",
	},
}

func TestGetIndicesByAlias(t *testing.T) {
	for _, tt := range getIndicesByAliasTests {
		t.Run("Should successfully delete index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newTestClient(ts.URL)
			resp, err := es.getIndicesByAlias(ctx, tt.alias)
			if !compareErrs(tt.err, err) {
				t.Fatalf("Unexpected error wanted: %v got: %v\n", tt.err, err)
			}
			if !reflect.DeepEqual(resp, tt.resp) {
				t.Fatalf("Index creation should have failed with error: %v got: %v \n", tt.resp, resp)
			}
		})
	}
}

var getMappingsTests = []struct {
	setup *ServerSetup
	index string
	err   string
}{
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test/_mapping/_all",
			Body:     "",
			Response: `{"test":{"mappings":{"_all":{"properties":{"counter":{"type":"long"},"field1":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}},"field2":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}},"tags":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}}}}}}}`,
		},
		"test",
		"",
	},
}

func TestMappingsOf(t *testing.T) {
	for _, tt := range getMappingsTests {
		t.Run("Should successfully delete index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newTestClient(ts.URL)
			_, err := es.mappingsOf(ctx, tt.index)
			if !compareErrs(tt.err, err) {
				t.Fatalf("Index creation should have failed with error: %v got: %v instead\n", tt.err, err)
			}
		})
	}
}

var getSettingsTests = []struct {
	setup *ServerSetup
	index string
	err   string
}{
	{
		&ServerSetup{
			Method:   "GET",
			Path:     "/test/_settings",
			Body:     "",
			Response: `{"test":{"settings":{"index":{"creation_date":"1552665579942","number_of_shards":"5","number_of_replicas":"1","uuid":"hqhO4oiCReawwtOqFHaVLA","version":{"created":"6020499"},"provided_name":"test"}}}}`,
		},
		"test",
		"",
	},
}

func TestSettingsOf(t *testing.T) {
	for _, tt := range getSettingsTests {
		t.Run("Should successfully delete index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			ts := buildTestServer(t, []*ServerSetup{tt.setup})
			defer ts.Close()
			es, _ := newTestClient(ts.URL)
			_, err := es.settingsOf(ctx, tt.index)
			if !compareErrs(tt.err, err) {
				t.Fatalf("Index creation should have failed with error: %v got: %v instead\n", tt.err, err)
			}

		})
	}
}

var reindexTests = []struct {
	alias string
	wait  bool
	resp  []string
	err   string
}{
	{
		"test",
		false,
		[]string{},
		"",
	},
	{
		"test",
		true,
		[]string{},
		"",
	},
}

func TestReindex(t *testing.T) {
	ss := []*ServerSetup{
		&ServerSetup{
			Method:   "POST",
			Path:     "/_reindex",
			Body:     `{"dest":{"index":"test_reindexed_1"},"source":{"_source":{"excludes":["test"],"includes":["test"]},"index":"test","type":"test"}}`,
			Response: `{"took":87,"timed_out":false,"total":2,"updated":2,"created":0,"deleted":0,"batches":1,"version_conflicts":0,"noops":0,"retries":{"bulk":0,"search":0},"throttled_millis":0,"requests_per_second":-1.0,"throttled_until_millis":0,"failures":[]}`,
		},
		&ServerSetup{
			Method:   "GET",
			Path:     "/_tasks/",
			Body:     "",
			Response: `{"took":87,"timed_out":false,"total":2,"updated":2,"created":0,"deleted":0,"batches":1,"version_conflicts":0,"noops":0,"retries":{"bulk":0,"search":0},"throttled_millis":0,"requests_per_second":-1.0,"throttled_until_millis":0,"failures":[]}`,
		},
	}

	config := &reindexConfig{
		Mappings: nil,
		Settings: nil,
		Include:  []string{"test"},
		Exclude:  []string{"test"},
		Types:    []string{"test"},
	}

	ts := buildTestServer(t, ss)
	defer ts.Close()

	for _, tt := range reindexTests {
		t.Run("Should successfully delete index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			es, _ := newTestClient(ts.URL)
			mock := &mockES{}

			_, err := es.reindex(ctx, mock, "test", config, tt.wait)
			if !compareErrs(tt.err, err) {
				t.Fatalf("Index creation should have failed with error, wanted: %v got: %v\n", tt.err, err)
			}
		})
	}
}

var reindexTestsErr = []struct {
	mock  reindexService
	alias string
	err   string
}{
	{
		&mockESIndicesByAliasErr{},
		"test",
		"multiple indices pointing to alias \"test\"",
	},
	{
		&mockESMappingsOfErr{},
		"test",
		"error fetching mappings of index \"test\": mappings result for index \"test\" not found",
	},
	{
		&mockESSettingsOfErr{},
		"test",
		"error fetching settings of index \"test\": settings for index test not found",
	},
	{
		&mockESCreateIndexErr{},
		"test",
		"failed to create index named \"test_reindexed_1\", acknowledged=false",
	},
	{
		&mockESDeleteIndexErr{},
		"test",
		"error deleting index \"test\": error deleting index \"test\", acknowledged=false\\n",
	},
	{
		&mockESAliasesOfErr{},
		"test",
		"error fetching aliases of index \"test\": elastic: cannot get connection from pool",
	},
	{
		&mockESSetAliasErr{},
		"test",
		"error setting alias \"test\" for index \"test_reindexed_1\"",
	},
}

func TestReindexErr(t *testing.T) {
	ss := &ServerSetup{
		Method:   "POST",
		Path:     "/_reindex",
		Body:     `{"dest":{"index":"test_reindexed_1"},"source":{"_source":{"excludes":["test"],"includes":["test"]},"index":"test","type":"test"}}`,
		Response: `{"took":87,"timed_out":false,"total":2,"updated":2,"created":0,"deleted":0,"batches":1,"version_conflicts":0,"noops":0,"retries":{"bulk":0,"search":0},"throttled_millis":0,"requests_per_second":-1.0,"throttled_until_millis":0,"failures":[]}`,
	}

	config := &reindexConfig{
		Mappings: nil,
		Settings: nil,
		Include:  []string{"test"},
		Exclude:  []string{"test"},
		Types:    []string{"test"},
	}

	ts := buildTestServer(t, []*ServerSetup{ss})
	defer ts.Close()

	for _, tt := range reindexTestsErr {
		t.Run("Should successfully delete index with a valid setup", func(t *testing.T) {
			ctx := context.Background()
			es, _ := newTestClient(ts.URL)

			_, err := es.reindex(ctx, tt.mock, "test", config, true)
			if !compareErrs(tt.err, err) {
				t.Fatalf("Reindexing should have failed with error, wanted: %v got: %v\n", tt.err, err)
			}
		})
	}
}
