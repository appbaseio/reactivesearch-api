package cache

import (
	"encoding/json"

	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type CacheSyncScript struct {
	index string
}

func (s CacheSyncScript) Index() string {
	return s.index
}
func (s CacheSyncScript) PluginName() string {
	return singleton.Name()
}

func (s CacheSyncScript) SetCache(response *elastic.SearchResult) error {
	cacheHits := util.GetHitsForIndex(response, s.index)
	for _, hit := range cacheHits {
		if hit.Id == cacheConfigDocID {
			var record CacheConfig
			err := json.Unmarshal(hit.Source, &record)
			if err != nil {
				log.Errorln(logTag, ": ", err)
				return err
			}
			setCachePreferences(record)
			return nil
		}
	}
	return nil
}
