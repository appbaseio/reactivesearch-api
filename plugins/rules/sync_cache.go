package rules

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
	rulesHits := util.GetHitsForIndex(response, s.index)

	var rules []ESRuleDoc
	for _, rule := range rulesHits {
		var esRule ESRuleDoc
		err := json.Unmarshal(rule.Source, &esRule)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		rules = append(rules, esRule)
	}
	// Update rules cache
	SetRulesToCache(rules)

	return nil
}
