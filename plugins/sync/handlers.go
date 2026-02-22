package sync

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

func (s *Sync) savePreferencesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		defer req.Body.Close()
		var syncConfig SyncPreferences
		err := json.NewDecoder(req.Body).Decode(&syncConfig)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf("Can't parse request body: %v", err), http.StatusBadRequest)
			return
		}
		if syncConfig.Interval == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "Interval must present", http.StatusBadRequest)
			return
		}
		if syncConfig.Interval != nil &&
			(*syncConfig.Interval < 10 || *syncConfig.Interval > 3600) {
			telemetry.WriteBackErrorWithTelemetry(req, w, "Interval must be in between [10, 3600] seconds.", http.StatusBadRequest)
			return
		}
		// store to ES
		res, err2 := s.es.saveSyncPreferences(req.Context(), syncConfig)
		if err2 != nil {
			status := http.StatusInternalServerError
			if res != nil {
				status = res.Status
			}
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), status)
			return
		}
		// Update util
		setPreferences(syncConfig)
		telemetry.WriteBackErrorWithTelemetry(req, w, "Sync preferences saved successfully", http.StatusOK)
	}
}

func (s *Sync) getPreferencesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record, err := s.es.getSyncPreferences(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, response, http.StatusOK)
	}
}
