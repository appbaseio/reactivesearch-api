package searchrelevancy

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
	return instance.Name()
}

func (s CacheSyncScript) SetCache(response *elastic.SearchResult) error {
	relevancyHits := util.GetHitsForIndex(response, s.index)
	var relevancySettings = map[string]SearchRelevancyStruct{}
	for _, hit := range relevancyHits {
		var relevancySetting SearchRelevancyStruct
		err := json.Unmarshal(hit.Source, &relevancySetting)
		if err != nil {
			log.Errorln(logTag, ": error while unmarshalling search relevancy record:", err)
			return err
		} else {
			relevancySettings[hit.Id] = relevancySetting
		}
	}
	// Update search relevancy
	SetSearchRelevancySettingsCache(relevancySettings)
	return nil
}
