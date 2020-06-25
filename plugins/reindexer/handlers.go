package reindexer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/util"
	"github.com/gorilla/mux"
)

type reindexConfig struct {
	Mappings                map[string]interface{}  `json:"mappings"`
	Settings                map[string]interface{}  `json:"settings"`
	SearchRelevancySettings *map[string]interface{} `json:"search_relevancy_settings"`
	Include                 []string                `json:"include_fields"`
	Exclude                 []string                `json:"exclude_fields"`
	Types                   []string                `json:"types"`
	Action                  []string                `json:"action,omitempty"`
}

func (rx *reindexer) reindex() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ok := vars["index"]
		if checkVar(ok, w, "index") {
			return
		}
		if IsReIndexInProcess(indexName, "") {
			util.WriteBackError(w, fmt.Sprintf(`Re-indexing is already in progress for %s index`, indexName), http.StatusInternalServerError)
			return
		}
		err, body, waitForCompletion, done := reindexConfigResponse(req, w, indexName)
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
		err, body, waitForCompletion, done := reindexConfigResponse(req, w, sourceIndex)
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

func reindexConfigResponse(req *http.Request, w http.ResponseWriter, sourceIndex string) (error, reindexConfig, bool, bool) {
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

	// By default, wait_for_completion depends on size of index
	param := req.URL.Query().Get("wait_for_completion")
	if param == "" {
		// Get the size of currentIndex, if that is > IndexStoreSize (100MB - 100000000 Bytes)  then do async re-indexing.
		size, err := getIndexSize(req.Context(), sourceIndex)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Unable to get the size of "+sourceIndex, http.StatusBadRequest)
			return nil, reindexConfig{}, false, true
		}
		if size > IndexStoreSize {
			param = "false"
		} else {
			param = "true"
		}
	}
	waitForCompletion, err := strconv.ParseBool(param)
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, err.Error(), http.StatusBadRequest)
		return nil, reindexConfig{}, false, true
	}
	return err, body, waitForCompletion, false
}
