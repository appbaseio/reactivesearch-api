package searchgrader

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/plugins"
)

func (s *SearchGrader) routes() []plugins.Route {
	middleware := (&chain{}).Wrap
	return []plugins.Route{
		{
			Name:        "Retrieve the grade evaluation metrics",
			Methods:     []string{http.MethodPost},
			Path:        "/_grade/metrics",
			HandlerFunc: middleware(s.postGradeMetrics()),
			Description: "This endpoint can be used to retrieve the grade evaluation metrics",
		},
		{
			Name:        "Grade a document",
			Methods:     []string{http.MethodPost},
			Path:        "/_grade/{index}/{doc_id}",
			HandlerFunc: middleware(s.postGrade()),
			Description: "This endpoint can be used to grade documents for a given index",
		},
		{
			Name:        "Retrieve the graded documents",
			Methods:     []string{http.MethodGet},
			Path:        "/_grade/{query}",
			HandlerFunc: middleware(s.getGradedDocuments()),
			Description: "This endpoint can be used to retrieve the graded documents with grade values",
		},
	}
}
