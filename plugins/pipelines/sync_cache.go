package pipelines

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
	pipelinessHits := util.GetHitsForIndex(response, s.index)

	var pipelines []ESPipelineDoc
	for _, pipeline := range pipelinessHits {
		var esPipeline ESPipelineDoc
		err := json.Unmarshal(pipeline.Source, &esPipeline)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return err
		}
		pipelines = append(pipelines, esPipeline)
	}
	// Update rules cache
	SetPipelinesToCache(pipelines)

	return nil
}
