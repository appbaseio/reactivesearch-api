package util

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// envMetaIndexPrefix allows forcing an alternate prefix for the internal
// (meta) indices, e.g. RS_META_INDEX_PREFIX=rs_ maps `.pipelines` to
// `rs_pipelines`. When unset, the prefix is applied automatically on
// Elasticsearch Serverless clusters where dot-prefixed index names are
// not allowed.
const envMetaIndexPrefix = "RS_META_INDEX_PREFIX"

// defaultServerlessMetaIndexPrefix is the prefix automatically applied to
// meta index names on Elasticsearch Serverless. Note: index names cannot
// start with `_`, `-` or `+`, so the prefix must start with a letter.
const defaultServerlessMetaIndexPrefix = "rs_"

var (
	serverlessOnce sync.Once
	serverlessFlag bool
)

// IsServerless reports whether the upstream cluster is an Elasticsearch
// Serverless project (detected via `version.build_flavor` from GET /).
// The result is cached after the first successful lookup.
func IsServerless() bool {
	serverlessOnce.Do(func() {
		requestOptions := es7.PerformRequestOptions{
			Method: "GET",
			Path:   "/",
		}
		response, err := GetClient7().PerformRequest(context.Background(), requestOptions)
		if err != nil {
			log.Warnln("error while detecting cluster build flavor: ", err)
			return
		}
		if response.StatusCode != http.StatusOK {
			log.Warnln("error while detecting cluster build flavor, got non OK status: ", response.StatusCode)
			return
		}

		statusMap := make(map[string]interface{})
		if unmarshalErr := json.Unmarshal(response.Body, &statusMap); unmarshalErr != nil {
			log.Warnln("error while detecting cluster build flavor: ", unmarshalErr)
			return
		}

		versionMap, ok := statusMap["version"].(map[string]interface{})
		if !ok {
			return
		}
		buildFlavor, _ := versionMap["build_flavor"].(string)
		serverlessFlag = buildFlavor == "serverless"
		if serverlessFlag {
			log.Infoln("Detected Elasticsearch Serverless cluster, meta indices will use the '" + metaIndexPrefix() + "' prefix instead of the '.' prefix")
		}
	})
	return serverlessFlag
}

// metaIndexPrefix returns the prefix to use in place of the leading dot
// for meta index names.
func metaIndexPrefix() string {
	if prefix := strings.TrimSpace(os.Getenv(envMetaIndexPrefix)); prefix != "" {
		return prefix
	}
	return defaultServerlessMetaIndexPrefix
}

// metaIndexRenamingActive reports whether meta index names should be
// rewritten: either explicitly requested via RS_META_INDEX_PREFIX or
// automatically on Elasticsearch Serverless.
func metaIndexRenamingActive() bool {
	if strings.TrimSpace(os.Getenv(envMetaIndexPrefix)) != "" {
		return true
	}
	return IsServerless()
}

// MetaIndexName resolves the actual name to use for an internal (meta)
// index. Dot-prefixed defaults like `.pipelines` are rewritten to a
// cluster-safe name (e.g. `rs_pipelines`) on Elasticsearch Serverless or
// when RS_META_INDEX_PREFIX is set. Names that don't begin with a dot
// (e.g. user-provided overrides) are returned unchanged.
func MetaIndexName(name string) string {
	if !strings.HasPrefix(name, ".") {
		return name
	}
	if !metaIndexRenamingActive() {
		return name
	}
	return metaIndexPrefix() + strings.TrimPrefix(name, ".")
}

// settings that Elasticsearch Serverless rejects (or that are pointless
// there) when creating an index.
var serverlessUnsupportedSettings = []string{
	"index.hidden",
	"index.number_of_shards",
	"index.number_of_replicas",
	"index.auto_expand_replicas",
	"hidden",
	"number_of_shards",
	"number_of_replicas",
	"auto_expand_replicas",
}

// AdaptIndexBody adapts an index creation body (or bare settings object)
// for the target cluster. On Elasticsearch Serverless it strips settings
// that are managed by the platform and rejected on index creation, such
// as shard/replica counts and `index.hidden`. On other clusters the body
// is returned unchanged.
func AdaptIndexBody(body string) string {
	if !IsServerless() {
		return body
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		log.Warnln("error while adapting index body for serverless, returning unchanged: ", err)
		return body
	}

	stripUnsupportedSettings(parsed)
	if settings, ok := parsed["settings"].(map[string]interface{}); ok {
		stripUnsupportedSettings(settings)
	}

	adapted, err := json.Marshal(parsed)
	if err != nil {
		log.Warnln("error while adapting index body for serverless, returning unchanged: ", err)
		return body
	}
	return string(adapted)
}

func stripUnsupportedSettings(settings map[string]interface{}) {
	for _, key := range serverlessUnsupportedSettings {
		delete(settings, key)
	}
	if index, ok := settings["index"].(map[string]interface{}); ok {
		for _, key := range serverlessUnsupportedSettings {
			delete(index, key)
		}
	}
}

// RolloverIndexSettings returns index settings for a rollover new-index template,
// adapted for the target cluster. On Elasticsearch Serverless, platform-managed
// settings (shards, replicas, hidden) are stripped via AdaptIndexBody. Rollover
// conditions (max_age, max_docs, max_size) must be evaluated client-side on
// Serverless via WriteIndexMeetsRolloverConditions before calling the rollover API.
func RolloverIndexSettings(numberOfShards int) map[string]interface{} {
	body := AdaptIndexBody(fmt.Sprintf(`{%s "index.number_of_shards": %d, "index.number_of_replicas": %d}`, HiddenIndexSettings(), numberOfShards, GetReplicas()))
	settings := make(map[string]interface{})
	if err := json.Unmarshal([]byte(body), &settings); err != nil {
		log.Warnln("error while parsing rollover index settings, using empty settings: ", err)
		return map[string]interface{}{}
	}
	return settings
}
