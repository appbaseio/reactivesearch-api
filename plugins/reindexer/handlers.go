package reindexer

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"

	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
)

type reindexConfig struct {
	Mappings map[string]interface{} `json:"mappings"`
	Settings map[string]interface{} `json:"settings"`
	Include  []string               `json:"include_fields"`
	Exclude  []string               `json:"exclude_fields"`
	Types    []string               `json:"types"`
}

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

		var body reindexConfig
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}

		// By default, wait_for_completion = true
		param := req.URL.Query().Get("wait_for_completion")
		if param == "" {
			param = "true"
		}
		waitForCompletion, err := strconv.ParseBool(param)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		response, err := rx.es.reindex(req.Context(), rx.es, indexName, &body, waitForCompletion)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, response, http.StatusOK)
	}
}
