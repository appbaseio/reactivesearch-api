package searchgrader

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func (s *SearchGrader) postGrade() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName := vars["index"]
		documentID := vars["doc_id"]

		var requestBody GradeRequest
		d := json.NewDecoder(req.Body)
		err := d.Decode(&requestBody)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		// validate request body
		if requestBody.Query == nil {
			util.WriteBackError(w, "Field `query` must be present", http.StatusBadRequest)
			return
		}

		if requestBody.Grade == nil {
			util.WriteBackError(w, "Field `grade` must be present", http.StatusBadRequest)
			return
		}
		if !(*requestBody.Grade >= 0 && *requestBody.Grade <= 10) {
			util.WriteBackError(w, "Grade value must be between `0` and `10`", http.StatusBadRequest)
			return
		}

		// convert to lowercase before save
		query := strings.ToLower(*requestBody.Query)

		record := ESRecord{
			Index: indexName,
			Query: &query,
			DocID: &documentID,
			Grade: requestBody.Grade,
		}
		// Update ES
		err3 := s.es.updateGrade(req.Context(), record)
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			util.WriteBackError(w, err3.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackMessage(w, "Graded document successfully", http.StatusAccepted)
	}
}

func (s *SearchGrader) postGradeMetrics() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var requestBody GradeMetricsRequest
		d := json.NewDecoder(req.Body)
		err := d.Decode(&requestBody)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		if len(requestBody.Indices) == 0 {
			util.WriteBackError(w, "Field `indices` can not be empty", http.StatusBadRequest)
			return
		}

		if len(requestBody.Indices) > 5 {
			util.WriteBackError(w, "You can compare maximum 5 indices at a time", http.StatusBadRequest)
			return
		}

		if requestBody.Page != nil && *requestBody.Page <= 0 {
			util.WriteBackError(w, "Field `page` must be greater than zero", http.StatusBadRequest)
			return
		}

		response, errorCode, err := s.es.getMetrics(req.Context(), requestBody)
		if err != nil {
			log.Errorln(logTag, ":", err)
			statusCode := http.StatusInternalServerError
			if errorCode != nil {
				statusCode = *errorCode
			}
			util.WriteBackError(w, err.Error(), statusCode)
			return
		}
		marshalled, err := json.Marshal(response)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, marshalled, http.StatusOK)
	}
}

func (s *SearchGrader) getGradedDocuments() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		query := vars["query"]
		// convert query to lowercase
		query = strings.ToLower(query)

		// handle the empty query case
		if query == "empty_query" {
			query = ""
		}

		response, err := s.es.getDocuments(req.Context(), query)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		finalResponse := map[string]interface{}{
			"docs": response,
		}
		marshalled, err := json.Marshal(finalResponse)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, marshalled, http.StatusOK)
	}
}
