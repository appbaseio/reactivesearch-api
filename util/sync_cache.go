package util

import (
	"fmt"

	"github.com/olivere/elastic/v7"
)

type SyncPluginCache interface {
	// Plugin name
	PluginName() string
	// Index to retrieve data
	Index() string
	// Method to set the ES data to plugin cache
	SetCache(response *elastic.SearchResult) error
}

var syncScripts []SyncPluginCache

// Polling interval in seconds, accepted range in [10, 3600].
// Defaults to 60 (i.e. 1 minute)
var syncInterval *int

func GetSyncInterval() int {
	// Sync interval is 60s for multi-tenant images
	if MultiTenant {
		return 60
	}
	if syncInterval != nil {
		return *syncInterval
	}
	if Billing == "true" {
		// For Arc Enterprise plan
		if GetTier(nil) != nil && *GetTier(nil) == ArcEnterprise {
			return 60
		}
	}
	// default range is 24h
	return 24 * 60 * 60
}

func SetSyncInterval(interval int) error {
	if interval < 10 || interval > 3600 {
		return fmt.Errorf("interval must be in range of [10, 3600] seconds")
	}
	syncInterval = &interval
	return nil
}

func GetSyncScripts() []SyncPluginCache {
	return syncScripts
}

// AddSyncScript allows you to add a sync cache script
func AddSyncScript(syncScript SyncPluginCache) {
	syncScripts = append(syncScripts, syncScript)
}

// Filters ES hits by index name
func GetHitsForIndex(response *elastic.SearchResult, index string) []*elastic.SearchHit {
	var hits = []*elastic.SearchHit{}
	for _, hit := range response.Hits.Hits {
		if hit.Index == index {
			hits = append(hits, hit)
		}
	}
	return hits
}
