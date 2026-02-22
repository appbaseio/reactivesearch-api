package openai

import (
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/util"
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
		if hit.Id == openAIConfigDocID {
			var record OpenAIConfig
			err := json.Unmarshal(hit.Source, &record)
			if err != nil {
				log.Errorln(logTag, ": ", err)
				return err
			}

			// Set the config in the local cache
			Instance().SetConfig(record)

			return nil
		}
	}
	return nil
}
