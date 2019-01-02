package reindexer

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/util"
	"github.com/gorilla/mux"
)

func (rx *reindexer) reindex() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ok := vars["index"]
		if !ok {
			util.WriteBackError(w, "Route inconsistency, expecting var {index}", http.StatusInternalServerError)
			return
		}

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var body struct {
			Mappings map[string]interface{} `json:"mappings"`
			Settings map[string]interface{} `json:"settings"`
			Include  []string               `json:"include_fields"`
			Exclude  []string               `json:"exclude_fields"`
			Types    []string               `json:"types"`
		}
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		err = rx.es.reindex(req.Context(), indexName, body.Mappings, body.Settings, body.Include, body.Exclude, body.Types)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusNotFound)
			return
		}

		util.WriteBackMessage(w, "Reindex successful", http.StatusOK)
	}
}
