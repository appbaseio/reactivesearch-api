package preferences

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/gorilla/mux"
)

func savePreferencesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ok := vars["index"]
		if !ok {
			util.WriteBackError(w, "Route inconsistency, expecting var {index}", http.StatusInternalServerError)
			return
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var body map[string]interface{}
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		response, err := savePreferences(req.Context(), indexName, body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, response, http.StatusOK)
	}
}

func getPreferencesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ok := vars["index"]
		if !ok {
			util.WriteBackError(w, "Route inconsistency, expecting var {index}", http.StatusInternalServerError)
			return
		}
		response, err := getPreferences(req.Context(), indexName)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, response, http.StatusOK)
	}
}
