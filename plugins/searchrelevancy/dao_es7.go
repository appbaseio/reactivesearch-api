package searchrelevancy

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) getRelevancySettingsEs7(ctx context.Context) (map[string]SearchRelevancyStruct, error) {
	// Size is set to 10000 temporarily, under assumption that there are maximum 10000 indices
	response, err := util.GetClient7().
		Search().
		Index(es.searchRelevancyIndex).
		Size(10000).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving searchrelvancy records:", err)
		return nil, err
	}

	var relevancySettings = map[string]SearchRelevancyStruct{}

	for _, hit := range response.Hits.Hits {
		var relevancySetting SearchRelevancyStruct
		err := json.Unmarshal(hit.Source, &relevancySetting)
		if err != nil {
			log.Errorln(logTag, ": error while unmarshalling searchrelvancy record:", err)
		} else {
			relevancySettings[hit.Id] = relevancySetting
		}
	}

	return relevancySettings, nil
}
