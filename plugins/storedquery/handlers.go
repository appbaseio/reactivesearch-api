package storedquery

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

func (s *StoredQuery) putStoredQuery() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		storedQueryID := vars["id"]
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}

		defer req.Body.Close()

		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			var requestBodyES ESStoredQueryDOC
			err2 := json.Unmarshal(reqBody, &requestBodyES)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// Update Cache
			err := AddStoredQueryToCache(requestBodyES)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			util.WriteBackMessage(w, "Query is updated successfully", http.StatusOK)
			return
		}

		var requestBody ESStoredQueryRequestBody
		err2 := json.Unmarshal(reqBody, &requestBody)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
			return
		}
		var query string

		if requestBody.Query != nil {
			queryInBytes, err := json.Marshal(*requestBody.Query)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
				return
			}
			query = string(queryInBytes)
		}

		var createdAt int64
		var updatedAt int64

		storedQuery, _ := IsStoredQueryExistsInCache(storedQueryID)

		if storedQuery != nil {
			createdAt = *storedQuery.CreatedAt
			// if stored query found then add `updated_at` property
			updatedAt = time.Now().Unix()
		} else {
			createdAt = time.Now().Unix()
		}

		var requestBodyES = ESStoredQueryDOC{
			ID:          &storedQueryID,
			Index:       requestBody.Index,
			Description: requestBody.Description,
			Query:       &query,
			Params:      requestBody.Params,
			CreatedAt:   &createdAt,
			UpdatedAt:   &updatedAt,
		}

		// Validate request body
		validateErr := validateStoredQuery(requestBodyES)
		if validateErr != nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, validateErr.Error(), http.StatusBadRequest)
			return
		}

		// Update ES
		err4 := s.es.updateStoredQuery(req.Context(), storedQueryID, requestBodyES)
		if err4 != nil {
			log.Errorln(logTag, ":", err4)
			telemetry.WriteBackErrorWithTelemetry(req, w, err4.Error(), http.StatusInternalServerError)
			return
		}
		// Invoke ACCAPI
		marshalledRequestBody, err5 := json.Marshal(requestBodyES)
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
				URL:    "/_storedquery/" + storedQueryID,
				Body:   bodyJSON, // forward body
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating storedquery")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update Cache
			err := AddStoredQueryToCache(requestBodyES)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		util.WriteBackMessage(w, "Query is updated successfully", http.StatusOK)
	}
}

func (s *StoredQuery) deleteStoredQuery() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		storedQueryID := vars["id"]
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update Cache
			ok := DeleteStoredQueryToCache(storedQueryID)
			if !ok {
				msg := "Error encountered while deleting the query"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
			util.WriteBackMessage(w, "Query is deleted successfully", http.StatusOK)
			return
		}
		// Delete query from ES
		err := s.es.deleteStoredQuery(req.Context(), storedQueryID)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Invoke ACCAPI
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodDelete,
				URL:    "/_storedquery/" + storedQueryID,
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered deleting query")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update Cache
			ok := DeleteStoredQueryToCache(storedQueryID)
			if !ok {
				msg := "Error encountered while deleting the query"
				log.Errorln(logTag, ":", msg)
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusInternalServerError)
				return
			}
		}
		util.WriteBackMessage(w, "Query is deleted successfully", http.StatusOK)
	}
}

func (s *StoredQuery) getStoredQueries() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		storedQueries := make([]ESStoredQueryGET, 0)

		cachedQueries := GetStoredQueriesFromCache()
		for _, query := range cachedQueries {
			storedQueries = append(storedQueries, ESStoredQueryGET{
				ID:          query.ID,
				Index:       query.Index,
				Description: query.Description,
				Query:       query.Query,
				Params:      query.Params,
				CreatedAt:   query.CreatedAt,
				UpdatedAt:   query.UpdatedAt,
			})
		}
		marshalledQueries, err := json.Marshal(storedQueries)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, marshalledQueries, http.StatusOK)
	}
}

func (s *StoredQuery) getStoredQuery() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

		vars := mux.Vars(req)
		storedQueryID := vars["id"]

		query := GetStoredQueryFromCache(storedQueryID)

		if query == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "stored query not found", http.StatusNotFound)
			return
		}

		esDocQuery := ESStoredQueryGET{
			ID:          query.ID,
			Index:       query.Index,
			Description: query.Description,
			Query:       query.Query,
			Params:      query.Params,
			CreatedAt:   query.CreatedAt,
			UpdatedAt:   query.UpdatedAt,
		}
		marshalledQueries, err := json.Marshal(esDocQuery)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, marshalledQueries, http.StatusOK)
	}
}

func (s *StoredQuery) validateStoredQueryID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var params *map[string]interface{}
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		if len(reqBody) > 0 {
			var requestBody ValidateQueryRequestBodyID
			err2 := json.Unmarshal(reqBody, &requestBody)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// params to apply
			params = requestBody.Params
		}

		// use query value from the stored query
		vars := mux.Vars(req)
		storedQueryID := vars["id"]
		storedQuery := GetStoredQueryFromCache(storedQueryID)
		if storedQuery == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "query not found", http.StatusNotFound)
			return
		}
		paramsToApply := getParams(storedQuery.Params, params)
		// apply request params
		params = &paramsToApply

		// validate params
		if params == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "params can not be empty", http.StatusBadRequest)
			return
		}

		queryInBytes, err := json.Marshal(*storedQuery.Query)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// parse query using params
		parsedQuery, err := renderQuery(queryInBytes, *params)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// validate query against ES
		isValid, err := s.es.validateQuery(req.Context(), parsedQuery)
		if isValid != nil && !*isValid {
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// write response back for valid query
		util.WriteBackRaw(w, []byte(parsedQuery), http.StatusOK)
	}
}

func (s *StoredQuery) validateStoredQuery() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var requestBody ValidateQueryRequestBody
		err2 := json.Unmarshal(reqBody, &requestBody)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
			return
		}

		// use query value from the stored query
		if requestBody.Query == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "query can not be empty", http.StatusBadRequest)
			return
		}

		// validate params
		if requestBody.Params == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "params can not be empty", http.StatusBadRequest)
			return
		}

		queryInBytes, err := json.Marshal(*requestBody.Query)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// parse query using params
		parsedQuery, err := renderQuery(queryInBytes, *requestBody.Params)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// validate query against ES
		isValid, err := s.es.validateQuery(req.Context(), parsedQuery)
		if isValid != nil && !*isValid {
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// write response back for valid query
		util.WriteBackRaw(w, []byte(parsedQuery), http.StatusOK)
	}
}

func (s *StoredQuery) executeStoredQueryID(params ...bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var params *map[string]interface{}
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		if len(reqBody) > 0 {
			var requestBody ExecuteQueryRequestBodyID
			err2 := json.Unmarshal(reqBody, &requestBody)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
				telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
				return
			}
			// params to apply
			params = requestBody.Params
		}

		// use query value from the stored query
		vars := mux.Vars(req)
		storedQueryID := vars["id"]
		storedQuery := GetStoredQueryFromCache(storedQueryID)
		if storedQuery == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "query not found", http.StatusNotFound)
			return
		}
		paramsToApply := getParams(storedQuery.Params, params)
		// apply request params
		params = &paramsToApply

		queryInBytes, err := json.Marshal(*storedQuery.Query)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// validate params
		if params == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "params can not be empty", http.StatusBadRequest)
			return
		}

		// parse query using params
		parsedQuery, err := renderQuery(queryInBytes, *params)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := s.es.executeQuery(req.Context(), *storedQuery.Index, parsedQuery)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// write response back for valid query
		util.WriteBackRaw(w, []byte(res), http.StatusOK)
	}
}

func (s *StoredQuery) executeStoredQuery(params ...bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var requestBody ExecuteQueryRequestBody
		err2 := json.Unmarshal(reqBody, &requestBody)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(req, w, err2.Error(), http.StatusBadRequest)
			return
		}

		// use query value from the stored query
		if requestBody.Query == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "query can not be empty", http.StatusBadRequest)
			return
		}

		// validate params
		if requestBody.Index == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "index can not be empty", http.StatusBadRequest)
			return
		}

		// validate params
		if requestBody.Params == nil {
			telemetry.WriteBackErrorWithTelemetry(req, w, "params can not be empty", http.StatusBadRequest)
			return
		}
		queryInBytes, err := json.Marshal(*requestBody.Query)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		// parse query using params
		parsedQuery, err := renderQuery(queryInBytes, *requestBody.Params)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		res, err := s.es.executeQuery(req.Context(), *requestBody.Index, parsedQuery)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}
		// write response back for valid query
		util.WriteBackRaw(w, []byte(res), http.StatusOK)
	}
}
