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
		err := s.featuredSuggestionsConfig.setFeaturedSuggestionsFromESResponse(response, s.index)
		if err != nil {
			return err
		}
	}
	return nil
}
