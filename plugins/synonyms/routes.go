package synonyms

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins"
)

func (s *Synonyms) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Get synonyms",
			Methods:     []string{http.MethodGet},
			Path:        "/_synonyms/{index}",
			HandlerFunc: middleware(s.getSynonyms()),
			Description: "Returns synonyms of an index",
		},
		{
			Name:        "Put synonyms",
			Methods:     []string{http.MethodPut},
			Path:        "/_synonyms/{index}",
			HandlerFunc: middleware(s.putSynonyms()),
			Description: "Saves synonyms of an index",
		},
		{
			Name:        "Delete synonyms",
			Methods:     []string{http.MethodDelete},
			Path:        "/_synonyms/{id}",
			HandlerFunc: middleware(s.deleteSynonyms()),
			Description: "Deletes synonyms of an index",
		},
		{
			Name:        "Delete All synonyms",
			Methods:     []string{http.MethodDelete},
			Path:        "/_synonyms_all/{index}",
			HandlerFunc: middleware(s.deleteAllSynonyms()),
			Description: "Deletes all synonyms of the passed index",
		},
	}
}
