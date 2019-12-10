package reindexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/olivere/elastic"
)

// Mock for successful reindexing with each ES operation returning a valid result.
type mockES struct {
}

func (m *mockES) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockES) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	data := []byte(`{"test":{"mappings":{"_doc":{"properties":{"counter":{"type":"long"},"field1":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}},"field2":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}},"tags":{"type":"text","fields":{"keyword":{"type":"keyword","ignore_above":256}}}}}}}}`)
	var dec map[string]interface{}
	_ = json.Unmarshal(data, &dec)

	result := dec[indexName]
	indexMappings, _ := result.(map[string]interface{})
	mappings, _ := indexMappings["mappings"]
	res, _ := mappings.(map[string]interface{})
	return res, nil
}

func (m *mockES) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	data := []byte(`{"test":{"settings":{"index":{"creation_date":"1552665579942","number_of_shards":"5","number_of_replicas":"1","uuid":"hqhO4oiCReawwtOqFHaVLA","version":{"created":"6020499"},"provided_name":"test"}}}}`)
	var dec map[string]*elastic.IndicesGetSettingsResponse
	_ = json.Unmarshal(data, &dec)

	result := dec[indexName]
	indexSettings, _ := result.Settings["index"].(map[string]interface{})

	settings := make(map[string]interface{})

	settings["index"] = make(map[string]interface{})
	settings["number_of_shards"] = indexSettings["number_of_shards"]
	settings["number_of_replicas"] = indexSettings["number_of_replicas"]
	analysis, found := result.Settings["analysis"]
	if found {
		settings["analysis"] = analysis
	}
	return settings, nil
}

func (m *mockES) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	return []string{"alias1", "alias2"}, nil
}

func (m *mockES) createIndex(ctx context.Context, name string, body map[string]interface{}) error {
	return nil
}

func (m *mockES) deleteIndex(ctx context.Context, name string) error {
	return nil
}

func (m *mockES) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return nil
}

func (m *mockES) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	return []string{"test"}, nil
}

// Mocks to test for various errors returned by internal fucntion calls in the reindex method.
// Each mock is used as a failing case for one of the service's methods.

// Test case for getIndicesByAlias failure
type mockESIndicesByAliasErr struct {
}

func (m *mockESIndicesByAliasErr) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockESIndicesByAliasErr) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESIndicesByAliasErr) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESIndicesByAliasErr) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

func (m *mockESIndicesByAliasErr) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	return nil
}

func (m *mockESIndicesByAliasErr) deleteIndex(ctx context.Context, indexName string) error {
	return nil
}

func (m *mockESIndicesByAliasErr) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return nil
}

func (m *mockESIndicesByAliasErr) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return []string{"test1", "test2"}, fmt.Errorf("elastic: cannot get connection from pool")
}

// Test case for mappingsOf failure
type mockESMappingsOfErr struct {
}

func (m *mockESMappingsOfErr) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockESMappingsOfErr) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, fmt.Errorf(`mappings result for index "%s" not found`, indexName)
}

func (m *mockESMappingsOfErr) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESMappingsOfErr) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

func (m *mockESMappingsOfErr) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	return nil
}

func (m *mockESMappingsOfErr) deleteIndex(ctx context.Context, indexName string) error {
	return nil
}

func (m *mockESMappingsOfErr) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return nil
}

func (m *mockESMappingsOfErr) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return []string{"test"}, nil
}

// Test case for settingsOf failure
type mockESSettingsOfErr struct {
}

func (m *mockESSettingsOfErr) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockESSettingsOfErr) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESSettingsOfErr) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, fmt.Errorf(`settings for index %s not found`, indexName)
}

func (m *mockESSettingsOfErr) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

func (m *mockESSettingsOfErr) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	return nil
}

func (m *mockESSettingsOfErr) deleteIndex(ctx context.Context, indexName string) error {
	return nil
}

func (m *mockESSettingsOfErr) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return nil
}

func (m *mockESSettingsOfErr) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

// Test case for aliasesOf failure
type mockESAliasesOfErr struct {
}

func (m *mockESAliasesOfErr) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockESAliasesOfErr) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESAliasesOfErr) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESAliasesOfErr) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, fmt.Errorf(`elastic: cannot get connection from pool`)
}

func (m *mockESAliasesOfErr) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	return nil
}

func (m *mockESAliasesOfErr) deleteIndex(ctx context.Context, indexName string) error {
	return nil
}

func (m *mockESAliasesOfErr) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return nil
}

func (m *mockESAliasesOfErr) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

// Test case for createIndex failure
type mockESCreateIndexErr struct {
}

func (m *mockESCreateIndexErr) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockESCreateIndexErr) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESCreateIndexErr) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESCreateIndexErr) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

func (m *mockESCreateIndexErr) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	return fmt.Errorf(`failed to create index named "%s", acknowledged=false`, indexName)
}

func (m *mockESCreateIndexErr) deleteIndex(ctx context.Context, indexName string) error {
	return nil
}

func (m *mockESCreateIndexErr) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return nil
}

func (m *mockESCreateIndexErr) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

// Test case for deleteIndex failure
type mockESDeleteIndexErr struct {
}

func (m *mockESDeleteIndexErr) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockESDeleteIndexErr) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESDeleteIndexErr) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESDeleteIndexErr) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

func (m *mockESDeleteIndexErr) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	return nil
}

func (m *mockESDeleteIndexErr) deleteIndex(ctx context.Context, indexName string) error {
	return fmt.Errorf(`error deleting index "%s", acknowledged=false`, indexName)
}

func (m *mockESDeleteIndexErr) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return nil
}

func (m *mockESDeleteIndexErr) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

// Test case for setAlias failure
type mockESSetAliasErr struct {
}

func (m *mockESSetAliasErr) reindex(ctx context.Context, index string, body *reindexConfig, waitForCompletion bool) ([]byte, error) {
	return nil, nil
}

func (m *mockESSetAliasErr) mappingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESSetAliasErr) settingsOf(ctx context.Context, indexName string) (map[string]interface{}, error) {
	return nil, nil
}

func (m *mockESSetAliasErr) aliasesOf(ctx context.Context, indexName string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}

func (m *mockESSetAliasErr) createIndex(ctx context.Context, indexName string, body map[string]interface{}) error {
	return nil
}

func (m *mockESSetAliasErr) deleteIndex(ctx context.Context, indexName string) error {
	return nil
}

func (m *mockESSetAliasErr) setAlias(ctx context.Context, indexName string, aliases ...string) error {
	return fmt.Errorf(`unable to set aliases "%v" for index "%s"`, aliases, indexName)
}

func (m *mockESSetAliasErr) getIndicesByAlias(ctx context.Context, alias string) ([]string, error) {
	// return some random ES error that might occur while making the request
	return nil, nil
}
