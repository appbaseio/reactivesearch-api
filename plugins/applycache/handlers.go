package applycache

import (
	"fmt"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins/analytics"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// getCacheAnalyticsHandler will get the cache analytics and
// handle extraction of the query params.
func (c *Cache) getCacheAnalyticsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// If `from` and `to` values are not passed then those will be set
		// in the following way:
		// from: 30 days from current day (including)
		// to: current day (including)
		rangeQueryParams := analytics.RangeQueryParams(req.URL.Query())

		rawResponse, analyticsErr := c.es.getCacheAnalytics(req.Context(), rangeQueryParams.From, rangeQueryParams.To)
		if analyticsErr != nil {
			errMsg := fmt.Sprint("error while getting analytics for cache, ", analyticsErr)
			log.Errorln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Write the response raw
		util.WriteBackRaw(w, rawResponse, http.StatusOK)
	}
}
