package validate

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/util"
)

// Elasticsearch returns a middleware that throws 404 for ES routes if external ES is not used
func Elasticsearch() middleware.Middleware {
	return validateES
}

func validateES(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if util.ExternalElasticsearch != "true" {
			util.WriteBackRaw(w, nil, http.StatusNotFound)
			return
		}
		h(w, req)
	}
}
