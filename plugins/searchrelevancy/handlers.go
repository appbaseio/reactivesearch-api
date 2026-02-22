package searchrelevancy

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gorilla/mux"
)

func (s *SearchRelevancy) getSearchRelevancySettings() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		index := vars["index"]
		if index == "" {
			log.Errorln(logTag, ": index is empty")
			util.WriteBackError(w, "Index is required", http.StatusBadRequest)
			return
		}

		var record SearchRelevancyStruct

		if index == "_default" {
			record = getDefaultRelevancySettings()
		} else {
			setting, err := GetSearchRelevancySettingFromCache(index)
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, "Settings not found", http.StatusNotFound)
				return
			}

			record = setting
		}

		// for the existing search relevancies it should return true for ngrams
		if record.IndexSettings == nil {
			record.IndexSettings = &IndexSettingStruct{
				EnableNgram: true,
			}
		}

		res, err := json.Marshal(record)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, res, http.StatusAccepted)

	}
}

func (s *SearchRelevancy) putSearchRelevancySettings() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		index := vars["index"]
		if index == "" {
			log.Errorln(logTag, ": index is empty")
			util.WriteBackError(w, "Index is required", http.StatusBadRequest)
			return
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}

		var record SearchRelevancyStruct
		err = json.Unmarshal(reqBody, &record)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// validate index name. check if index exists if not check if alias exists
		indexExists := util.CheckIfIndexExists(req.Context(), index)
		if !indexExists {
			log.Errorln(logTag, ": index not found")
			util.WriteBackError(w, "Invalid index name", http.StatusBadRequest)
			return
		}

		err = s.validate.Struct(record)
		if err != nil {
			log.Errorln(logTag, ": struct validation error", err.Error())
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Get currentSetting / default setting to set the missing data.
		existingData, err := GetSearchRelevancySettingFromCache(index)
		if err != nil {
			log.Errorln(logTag, ":", err)
			existingData = getDefaultRelevancySettings()
		}

		if record.Search == nil {
			record.Search = existingData.Search
		}

		if record.Aggregations == nil {
			record.Aggregations = existingData.Aggregations
		}

		if record.Results == nil {
			record.Results = existingData.Results
		}

		if record.Language == nil {
			record.Language = existingData.Language
		}

		if record.Synonyms == nil {
			record.Synonyms = existingData.Synonyms
		}

		if record.Rules == nil {
			record.Rules = existingData.Rules
		}

		// validate if fieldWeights is array of float
		for _, v := range record.Search.FieldWeights {
			strVal := fmt.Sprintf("%v", v)
			_, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update Cache
			UpdateSearchRelevancyCache(index, record)

			res, err := json.Marshal(record)
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
			util.WriteBackRaw(w, res, http.StatusAccepted)
			return
		}

		err2 := s.es.putSearchRelevancySettings(req.Context(), index, record)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			util.WriteBackError(w, err2.Error(), http.StatusInternalServerError)
			return
		}
		// Invoke ACCAPI
		var bodyJSON map[string]interface{}
		err3 := json.Unmarshal(reqBody, &bodyJSON)
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			util.WriteBackError(w, err3.Error(), http.StatusBadRequest)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			response, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPut,
				URL:    "/_searchrelevancy/" + index,
				Body:   bodyJSON,
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if response != nil {
				log.Errorln(logTag, ":", "error encountered updating search settings")
				bodyBytes, err := ioutil.ReadAll(response.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, response.StatusCode)
				return
			}
		} else {
			UpdateSearchRelevancyCache(index, record)
		}
		res, err := json.Marshal(record)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, res, http.StatusAccepted)
	}
}

func (s *SearchRelevancy) deleteSearchRelevancySettings() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		index := vars["index"]
		if index == "" {
			log.Errorln(logTag, ": index is empty")
			util.WriteBackError(w, "Index is required", http.StatusBadRequest)
			return
		}
		res := []byte(fmt.Sprintf(`
			{
				"_id": "%s",
				"acknowledge": true,
				"message": "Search settings deleted successfully"
			}
		`, index))
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update Cache
			RemoveFromSearchRelevancyCache(index)
			util.WriteBackRaw(w, res, http.StatusAccepted)
			return
		}

		// validate index name. check if index exists if not check if alias exists
		indexExists := util.CheckIfIndexExists(req.Context(), index)
		if !indexExists {
			log.Errorln(logTag, ": index not found")
			util.WriteBackError(w, "Invalid index name", http.StatusBadRequest)
			return
		}

		_ = s.es.deleteSearchRelevancySettings(req.Context(), index)
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Invoke ACCAPI
			response, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodDelete,
				URL:    "/_searchrelevancy/" + index,
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if response != nil {
				log.Errorln(logTag, ":", "error encountered deleting search settings")
				bodyBytes, err := ioutil.ReadAll(response.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, response.StatusCode)
				return
			}
		} else {
			// Update Cache
			RemoveFromSearchRelevancyCache(index)
		}
		util.WriteBackRaw(w, res, http.StatusAccepted)
	}
}
