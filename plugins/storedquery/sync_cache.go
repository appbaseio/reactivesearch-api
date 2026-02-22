package storedquery

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
	storedQueryHits := util.GetHitsForIndex(response, s.index)

	var storedQueries []ESStoredQueryDOC
	for _, rule := range storedQueryHits {
		var esRule ESStoredQueryDOC
		err := json.Unmarshal(rule.Source, &esRule)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		storedQueries = append(storedQueries, esRule)
	}
	// Update stored queries cache
	SetStoredQueriesToCache(storedQueries)

	return nil
}
