package classify

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/domain"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
)

// Indices returns a middleware that identifies the indices present in the es route.
func Indices() middleware.Middleware {
	return indices
}

func indices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indices := util.IndicesFromRequest(req)
		currentCache := GetIndexAliasCache()
		domainUsed, domainFetchErr := domain.FromContext(req.Context())
		if domainFetchErr != nil {
			errMsg := "Error while validating the domain!"
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusUnauthorized)
			return
		}
		tenantDetails := util.GetSLSInstanceByDomain(domainUsed.Raw)
		if tenantDetails == nil {
			errMsg := "Error while validating the domain!"
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusUnauthorized)
			return
		}
		tenantId := tenantDetails.TenantID
		for _, index := range indices {
			// '*' in case of all indices put alias in context
			if index == "*" {
				for cachedItem := range currentCache {
					alias := GetIndexAlias(tenantId, cachedItem)
					if alias != "" {
						indices = append(indices, alias)
					}
				}
				break
			} else if strings.Contains(index, "*") {
				// in case of regex check if string contains '*' in naming pattern, if contains and doesn't have '.*' and replace '*' with '.*' because golang regex can match in that pattern. Next match regex patters with existing index names in cache and add those alias to context.
				regex := index

				if !strings.Contains(index, ".*") {
					regex = strings.Replace(regex, "*", ".*", -1)
				}
				r, _ := regexp.Compile(regex)
				cachedIndices := []string{}

				for cachedItem := range currentCache {
					cachedIndices = append(cachedIndices, cachedItem)
				}
				for _, val := range cachedIndices {
					if r.MatchString(val) {
						alias := GetIndexAlias(tenantId, val)
						if alias != "" {
							indices = append(indices, alias)
						}
						break
					}
				}
			} else {
				// get alias for index and put in context
				alias := GetIndexAlias(tenantId, index)
				if alias != "" {
					indices = append(indices, alias)
				}
				break
			}
		}
		ctx := index.NewContext(req.Context(), indices)
		req = req.WithContext(ctx)

		h(w, req)
	}
}
