package elasticsearch

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// WhitelistedRoute will contain the path
// of the route
type WhitelistedRoute struct {
	Path string
}

// GetWhitelistedRoutesForSystem will return a map of path
// to the whitelisted methods allowed for that path
func GetWhitelistedRoutesForSystem() map[string][]string {
	return map[string][]string{
		"/{index}": {
			http.MethodGet, http.MethodPut,
		},
		"/{index}/_analyze": {
			http.MethodGet, http.MethodPost,
		},
		"/{index}/_mapping": {
			http.MethodGet, http.MethodPost, http.MethodPut,
		},
		"/{index}/_mappings": {
			http.MethodPost, http.MethodPut,
		},
		"/{index}/_delete_by_query": {
			http.MethodPost,
		},
		"/{index}/_recovery": {
			http.MethodGet,
		},
		"/{index}/_settings": {
			http.MethodGet, http.MethodPut,
		},
		"/{index}/_open": {
			http.MethodPost,
		},
		"/{index}/_close": {
			http.MethodPost,
		},
		"/{index}/_stats": {
			http.MethodGet,
		},
		"/{index}/_stats/{metric}": {
			http.MethodGet,
		},
		"/{index}/_count": {
			http.MethodPost, http.MethodGet,
		},
		"/{index}/{id}": {
			http.MethodGet,
		},
		"/{index}/_doc/{id}": {
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete,
		},
		"/{index}/_termvectors": {
			http.MethodGet, http.MethodPost,
		},
		"/{index}/_termvectors/{id}": {
			http.MethodGet, http.MethodPost,
		},
		"/{index}/_mget": {
			http.MethodGet, http.MethodPost,
		},
		"/{index}/_doc/{id}/_update": {
			http.MethodPost,
		},
		"/{index}/_bulk": {
			http.MethodPost, http.MethodPut,
		},
		"/{index}/_doc": {
			http.MethodPost, http.MethodPut,
		},
	}
}

// GetMethods will get the methods for the attached
// path.
func (w *WhitelistedRoute) GetMethods() []string {
	methods, exists := GetWhitelistedRoutesForSystem()[w.Path]
	if !exists {
		return make([]string, 0)
	}

	return methods
}

// IsMethodWhitelisted will check if the method is whitelisted
// for the path
func (w *WhitelistedRoute) IsMethodWhitelisted(methodPassed string) bool {
	for _, method := range w.GetMethods() {
		if methodPassed == method {
			return true
		}
	}

	return false
}

// CheckIfPathWhitelisted will check if the path being called is whitelisted
// based on the method and accordingly allow/deny access.
//
// This should be the first middleware that is called so that no other
// middleware are executed if the path is denied
func (wh *WhitelistedRoute) CheckIfPathWhitelisted(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// This check only needs to happen if the instance is
		// multi-tenant and backend for the incoming domain is `system`
		if util.IsSLSDisabled() || !util.MultiTenant {
			h(w, req)
			return
		}

		// Fetch the domain from context
		domainUsed, domainFetchErr := domain.FromContext(req.Context())
		if domainFetchErr != nil {
			errMsg := "Error while validating the domain!"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusUnauthorized)
			return
		}

		// Get the backend using the domain
		if *(util.GetBackendByDomain(domainUsed.Raw)) != util.System {
			// No need to blacklist
			h(w, req)
			return
		}

		// Check if the request method is whitelisted, else deny access
		if !wh.IsMethodWhitelisted(req.Method) {
			telemetry.WriteBackErrorWithTelemetry(req, w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		// Allow access otherwise
		h(w, req)
	}
}
