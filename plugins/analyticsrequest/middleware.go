package analyticsrequest

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/storedquery"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// Plugin to track cache
func trackAnalyticsRequestPlugin(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rsRequestBody, err := querytranslate.FromContext(r.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		// Only record when recordAnalytics is set to true
		if rsRequestBody.Settings == nil || rsRequestBody.Settings.RecordAnalytics == nil || !*rsRequestBody.Settings.RecordAnalytics {
			h(w, r)
			return
		}
		// ignore analytics for validate route
		if util.IsRSAPIValidateRoute(r) {
			h(w, r)
			return
		}
		// records the analytics request into a context
		envs := querytranslate.ExtractEnvsFromRequest(*rsRequestBody)

		// capture search query
		var searchQuery string
		if envs.Query != nil {
			searchQuery = *envs.Query
		} else {
			searchQuery = "" // empty query
		}

		// capture stored queries
		storedQueries, err := storedquery.FromContext(r.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "error encountered while retrieving stored queries from context", http.StatusInternalServerError)
			return
		}
		record, err := FromContext(r.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "error encountered while retrieving analytics record from context", http.StatusInternalServerError)
			return
		}
		// Modify the pointers so changes can reflect in the analytics response recorder
		*record.SearchQuery = searchQuery
		*record.StoredQueries = storedQueries
		ctx := NewContext(r.Context(), *record)
		r = r.WithContext(ctx)
		h(w, r)
	}
}
