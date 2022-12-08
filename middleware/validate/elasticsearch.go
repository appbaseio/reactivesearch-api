package validate

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/domain"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// Elasticsearch returns a middleware that validates SLS search backend to be ES or OS (Open search).
func Elasticsearch() middleware.Middleware {
	return validateElasticsearch
}

func validateElasticsearch(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// If SLS is disabled, just continue to the next handler, we don't
		// need to do anything here.
		if util.IsSLSDisabled() {
			h(w, req)
			return
		}

		// Else, if backend is `elasticsearch` or `system`, we can allow
		// access else deny.

		// Fetch the backed using the domain
		var backend *util.Backend
		if util.MultiTenant {
			// Fetch the domain from context
			domainUsed, domainFetchErr := domain.FromContext(req.Context())
			if domainFetchErr != nil {
				errMsg := "Error while validating the domain!"
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusUnauthorized)
				return
			}

			backend = util.GetBackendByDomain(domainUsed.Raw)
		} else {
			backend = util.GetBackend()
		}

		if *backend != util.ElasticSearch && *backend != util.OpenSearch && *backend != util.System {
			util.WriteBackRaw(w, nil, http.StatusNotFound)
			return
		}

		h(w, req)
	}
}
