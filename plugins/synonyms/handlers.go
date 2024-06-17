package synonyms

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/gorilla/mux"
)

func (s *Synonyms) getSynonyms() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		index := vars["index"]
		if index == "" {
			log.Errorln(logTag, ": index is empty")
			util.WriteBackError(w, "Index is required", http.StatusBadRequest)
			return
		}

		response, _ := s.es.getSynonyms(req.Context(), index)

		if response == nil {
			response = []SynonymsStruct{}
		}

		res, err := json.Marshal(response)

		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, res, http.StatusAccepted)
		return

	}
}

func (s *Synonyms) putSynonyms() http.HandlerFunc {
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

		var record []SynonymsStruct
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

		for _, rec := range record {
			err = s.validate.Struct(rec)
			if err != nil {
				log.Errorln(logTag, ": struct validation error", err.Error())
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		_, record = s.es.putSynonyms(req.Context(), record, index)

		res, err := json.Marshal(record)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}
		util.WriteBackRaw(w, res, http.StatusAccepted)
		return
	}
}

func (s *Synonyms) deleteSynonyms() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		id := vars["id"]
		if id == "" {
			log.Errorln(logTag, ": id is empty")
			util.WriteBackError(w, "Id is required", http.StatusBadRequest)
			return
		}

		err := s.es.deleteSynonyms(req.Context(), id)

		if err != nil {
			res := []byte(fmt.Sprintf("Error while deleting %s. %s", id, err.Error()))
			util.WriteBackRaw(w, res, http.StatusBadRequest)
		} else {
			res := []byte(fmt.Sprintf(`
			{
				"_id": "%s",
				"acknowledge": true,
				"message": "Synonyms deleted successfully"
			}
		`, id))
			util.WriteBackRaw(w, res, http.StatusAccepted)
		}
		return
	}
}

func (s *Synonyms) deleteAllSynonyms() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		index := vars["index"]
		if index == "" {
			log.Errorln(logTag, ": index is empty")
			util.WriteBackError(w, "Index is required", http.StatusBadRequest)
			return
		}

		// validate index name. check if index exists if not check if alias exists
		indexExists := util.CheckIfIndexExists(req.Context(), index)
		if !indexExists {
			log.Errorln(logTag, ": index not found")
			util.WriteBackError(w, "Invalid index name", http.StatusBadRequest)
			return
		}

		// Run a delete by query to delete all objects that contain a certain index
		deleteErr := s.es.deleteAllSynonyms(req.Context(), index)
		if deleteErr != nil {
			errMsg := fmt.Sprintf("error while deleting all synonyms of passed index: %s", deleteErr.Error())
			log.Warnln(logTag, errMsg)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackMessage(w, "Deleted Succesfully!", http.StatusOK)
		return
	}
}
