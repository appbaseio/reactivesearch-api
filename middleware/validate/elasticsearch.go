package validate

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/util"
)

// Elasticsearch returns a middleware that validates SLS search backend to be ES or OS (Open search).
func Elasticsearch() middleware.Middleware {
	return validateElasticsearch
}

func validateElasticsearch(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if util.IsSLSEnabled() {
			backend := util.GetBackend()
			if backend != nil {
				if *backend != util.ElasticSearch && *backend != util.OpenSearch {
					util.WriteBackRaw(w, nil, http.StatusNotFound)
					return
				}
			}
		}
		h(w, req)
	}
}
