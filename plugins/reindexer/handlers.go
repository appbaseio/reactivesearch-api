package reindexer

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
)

type reindexConfig struct {
	Mappings map[string]interface{} `json:"mappings"`
	Settings map[string]interface{} `json:"settings"`
	Include  []string               `json:"include_fields"`
	Exclude  []string               `json:"exclude_fields"`
	Types    []string               `json:"types"`
	Action   string                 `json:"action"`
}

func (rx *reindexer) reindex() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ok := vars["index"]
		if checkVar(ok, w, "index") {
			return
		}

		err, body, waitForCompletion, done := reindexConfigResponse(req, w)
		if done {
			return
		}

		response, err := reindex(req.Context(), indexName, &body, waitForCompletion, "")
		errorHandler(err, w, response)
	}
}

func (rx *reindexer) reindexSrcToDest() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		sourceIndex, okS := vars["source_index"]
		destinationIndex, okD := vars["destination_index"]
		if checkVar(okS, w, "source_index") {
			return
		}
		if checkVar(okD, w, "destination_index") {
			return
		}
		err, body, waitForCompletion, done := reindexConfigResponse(req, w)
		if done {
			return
		}

		response, err := reindex(req.Context(), sourceIndex, &body, waitForCompletion, destinationIndex)
		errorHandler(err, w, response)
	}
}

func (rx *reindexer) aliasedIndices() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		res, err := getAliasedIndices(req.Context())
		if err != nil {
			util.WriteBackError(w, "Unable to get aliased indices.\n"+err.Error(), http.StatusInternalServerError)
			return
		}

		response, err := json.Marshal(res)
		errorHandler(nil, w, response)
	}
}

func errorHandler(err error, w http.ResponseWriter, response []byte) {
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, err.Error(), http.StatusNotFound)
		return
	}

	util.WriteBackRaw(w, response, http.StatusOK)
}

func checkVar(okS bool, w http.ResponseWriter, variable string) bool {
	if !okS {
		util.WriteBackError(w, "Route inconsistency, expecting var "+variable, http.StatusInternalServerError)
		return true
	}
	return false
}

func reindexConfigResponse(req *http.Request, w http.ResponseWriter) (error, reindexConfig, bool, bool) {
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
		return nil, reindexConfig{}, false, true
	}
	defer req.Body.Close()

	var body reindexConfig
	err = json.Unmarshal(reqBody, &body)
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
		return nil, reindexConfig{}, false, true
	}

	// By default, wait_for_completion = true
	param := req.URL.Query().Get("wait_for_completion")
	if param == "" {
		param = "true"
	}
	waitForCompletion, err := strconv.ParseBool(param)
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, err.Error(), http.StatusBadRequest)
		return nil, reindexConfig{}, false, true
	}
	return err, body, waitForCompletion, false
}
