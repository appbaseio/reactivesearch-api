package suggestions

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/plugins/telemetry"
	"github.com/appbaseio-confidential/reactivesearch/util"
)

// Route handler to update the popular suggestions preferences
func (rx *suggestions) savePopularSuggestionsPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			var requestBodyES PopularPreferences
			err2 := json.Unmarshal(reqBody, &requestBodyES)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// Update Cache
			SetPopularPreferences(requestBodyES)

			util.WriteBackMessage(w, "Preferences saved successfully", http.StatusOK)
			return
		}

		var body PopularPreferences
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		// Update ES
		res, err1 := rx.esMeta.savePopularSuggestionsPreferences(req.Context(), body)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			status := http.StatusInternalServerError
			if res != nil {
				status = res.Status
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), status)
			return
		}

		// Invoke ACCAPI
		marshalledRequestBody, err5 := json.Marshal(body)
		if err5 != nil {
			log.Errorln(logTag, ":", err5)
			telemetry.WriteBackErrorWithTelemetry(req, w, err5.Error(), http.StatusInternalServerError)
			return
		}
		var bodyJSON map[string]interface{}
		err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
		if err6 != nil {
			log.Errorln(logTag, ":", err6)
			telemetry.WriteBackErrorWithTelemetry(req, w, err6.Error(), http.StatusInternalServerError)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_popular_suggestions/preferences",
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				status := http.StatusInternalServerError
				if res != nil {
					status = res.StatusCode
				}
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), status)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating popular suggestions preferences")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), res.StatusCode)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update Cache
			SetPopularPreferences(body)
			// Re-populate the popular suggestions index, no need to wait
			go func() {
				syncAnalyticsToSuggestionsZinc(rx, body.AliasToIndex)
			}()
		}

		util.WriteBackMessage(w, "Preferences saved successfully, re-populating the popular suggestions index ⏳", http.StatusOK)
	}
}

// Route handler to update the index suggestions preferences
func (rx *suggestions) saveIndexSuggestionsPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			var requestBodyES IndexPreferences
			err2 := json.Unmarshal(reqBody, &requestBodyES)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// Update Cache
			SetIndexPreferences(requestBodyES)

			util.WriteBackMessage(w, "Preferences saved successfully", http.StatusOK)
			return
		}

		var body IndexPreferences
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		// Update ES
		res, err1 := rx.esMeta.saveIndexSuggestionsPreferences(req.Context(), body)
		if err1 != nil {
			status := http.StatusInternalServerError
			if res != nil {
				status = res.Status
			}
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), status)
			return
		}

		// Invoke ACCAPI
		marshalledRequestBody, err5 := json.Marshal(body)
		if err5 != nil {
			log.Errorln(logTag, ":", err5)
			telemetry.WriteBackErrorWithTelemetry(req, w, err5.Error(), http.StatusInternalServerError)
			return
		}
		var bodyJSON map[string]interface{}
		err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
		if err6 != nil {
			log.Errorln(logTag, ":", err6)
			telemetry.WriteBackErrorWithTelemetry(req, w, err6.Error(), http.StatusInternalServerError)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause inconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_index_suggestions/preferences",
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				status := http.StatusInternalServerError
				if res != nil {
					status = res.StatusCode
				}
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), status)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating index suggestions preferences")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), res.StatusCode)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update Cache
			SetIndexPreferences(body)
		}

		util.WriteBackMessage(w, "Preferences saved successfully", http.StatusOK)
	}
}

// Route handler to update the recent suggestions preferences
func (rx *suggestions) saveRecentSuggestionsPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			var requestBodyES RecentPreferences
			err2 := json.Unmarshal(reqBody, &requestBodyES)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// Update Cache
			SetRecentPreferences(requestBodyES)

			util.WriteBackMessage(w, "Preferences saved successfully", http.StatusOK)
			return
		}

		var body RecentPreferences
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		// Update ES
		res, err1 := rx.esMeta.saveRecentSuggestionsPreferences(req.Context(), body)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			status := http.StatusInternalServerError
			if res != nil {
				status = res.Status
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), status)
			return
		}

		// Invoke ACCAPI
		marshalledRequestBody, err5 := json.Marshal(body)
		if err5 != nil {
			log.Errorln(logTag, ":", err5)
			telemetry.WriteBackErrorWithTelemetry(req, w, err5.Error(), http.StatusInternalServerError)
			return
		}
		var bodyJSON map[string]interface{}
		err6 := json.Unmarshal(marshalledRequestBody, &bodyJSON)
		if err6 != nil {
			log.Errorln(logTag, ":", err6)
			telemetry.WriteBackErrorWithTelemetry(req, w, err6.Error(), http.StatusInternalServerError)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_recent_suggestions/preferences",
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				status := http.StatusInternalServerError
				if res != nil {
					status = res.StatusCode
				}
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), status)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating recent suggestions preferences")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), res.StatusCode)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update Cache
			SetRecentPreferences(body)
		}

		util.WriteBackMessage(w, "Preferences saved successfully", http.StatusOK)
	}
}

// Route handler to retrieve the popular suggestions preferences
func (rx *suggestions) getPopularSuggestionsPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record := GetPopularPreferences()

		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusNotFound)
			return
		}

		suggestionsIndex := os.Getenv(envSuggestionsEsIndex)
		if suggestionsIndex == "" {
			suggestionsIndex = defaultSuggestionsEsIndex
		}

		var finalResponse map[string]interface{}

		json.Unmarshal(response, &finalResponse)

		finalResponse["index"] = suggestionsIndex

		marshalledRes, err1 := json.Marshal(finalResponse)

		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, marshalledRes, http.StatusOK)
	}
}

// Route handler to retrieve the index suggestions preferences
func (rx *suggestions) getIndexSuggestionsPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record := GetIndexPreferences()

		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

// Route handler to retrieve the recent suggestions preferences
func (rx *suggestions) getRecentSuggestionsPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		record := GetRecentPreferences()

		response, err1 := json.Marshal(record)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			telemetry.WriteBackErrorWithTelemetry(req, w, err1.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, response, http.StatusOK)
	}
}
