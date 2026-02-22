package suggestions

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
	suggestionsHits := util.GetHitsForIndex(response, s.index)
	for _, hit := range suggestionsHits {
		switch hit.Id {
		case popularPreferenceDocID:
			var record PopularPreferences
			err := json.Unmarshal(hit.Source, &record)
			if err != nil {
				log.Errorln(logTag, ": ", err)
				return err
			}
			// set popular preferences in cache
			SetPopularPreferences(record)
		case recentPreferenceDocID:
			var record RecentPreferences
			err := json.Unmarshal(hit.Source, &record)
			if err != nil {
				log.Errorln(logTag, ": ", err)
				return err
			}
			// set recent preferences in cache
			SetRecentPreferences(record)
		case indexPreferenceDocID:
			var record IndexPreferences
			err := json.Unmarshal(hit.Source, &record)
			if err != nil {
				log.Errorln(logTag, ": ", err)
				return err
			}
			// set index preferences in cache
			SetIndexPreferences(record)
		}
	}
	return nil
}
