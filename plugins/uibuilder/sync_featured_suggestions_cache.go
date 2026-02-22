package uibuilder

import (
	"github.com/olivere/elastic/v7"
)

type FeaturedSuggestionsCacheSyncScript struct {
	index                     string
	featuredSuggestionsConfig FeaturedSuggestionsConfig
}

func (s FeaturedSuggestionsCacheSyncScript) Index() string {
	return s.index
}
func (s FeaturedSuggestionsCacheSyncScript) PluginName() string {
	return singleton.Name()
}

func (s FeaturedSuggestionsCacheSyncScript) SetCache(response *elastic.SearchResult) error {
	if response != nil {
		// Always sync to ES when called from sync script (data has changed)
		err := s.featuredSuggestionsConfig.setFeaturedSuggestionsFromESResponse(response, s.index, true)
		if err != nil {
			return err
		}
	}
	return nil
}
