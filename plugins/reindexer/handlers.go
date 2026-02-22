package reindexer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/model/reindex"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gorilla/mux"
)

func (rx *reindexer) reindex() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ok := vars["index"]
		if checkVar(ok, req, w, "index") {
			return
		}
		if reindex.IsReIndexInProcess(indexName, "") {
			util.WriteBackError(w, fmt.Sprintf(`Re-indexing is already in progress for %s index`, indexName), http.StatusInternalServerError)
			return
		}
		err, body, waitForCompletion, done := reindexConfigResponse(req, w, indexName)
		if done {
			return
		}

		response, err := reindex.Reindex(req.Context(), indexName, &body, waitForCompletion, "")
		errorHandler(err, req, w, response)
	}
}

func (rx *reindexer) reindexSrcToDest() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		sourceIndex, okS := vars["source_index"]
		destinationIndex, okD := vars["destination_index"]
		if checkVar(okS, req, w, "source_index") {
			return
		}
		if checkVar(okD, req, w, "destination_index") {
			return
		}
		err, body, waitForCompletion, done := reindexConfigResponse(req, w, sourceIndex)
		if done {
			return
		}

		response, err := reindex.Reindex(req.Context(), sourceIndex, &body, waitForCompletion, destinationIndex)
		errorHandler(err, req, w, response)
	}
}

func (rx *reindexer) aliasedIndices() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		res, err := reindex.GetAliasedIndices(req.Context())
		if err != nil {
			util.WriteBackError(w, "Unable to get aliased indices.\n"+err.Error(), http.StatusInternalServerError)
			return
		}

		for _, aliasIndex := range res {
			if aliasIndex.Alias != "" {
				classify.SetIndexAlias(aliasIndex.Index, aliasIndex.Alias)
				classify.SetAliasIndex(aliasIndex.Alias, aliasIndex.Index)
			}
		}

		response, err := json.Marshal(res)
		errorHandler(nil, req, w, response)
	}
}

func errorHandler(err error, req *http.Request, w http.ResponseWriter, response []byte) {
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, err.Error(), http.StatusBadRequest)
		return
	}

	util.WriteBackRaw(w, response, http.StatusOK)
}

func checkVar(okS bool, req *http.Request, w http.ResponseWriter, variable string) bool {
	if !okS {
		util.WriteBackError(w, "Route inconsistency, expecting var "+variable, http.StatusInternalServerError)
		return true
	}
	return false
}

func reindexConfigResponse(req *http.Request, w http.ResponseWriter, sourceIndex string) (error, reindex.ReindexConfig, bool, bool) {
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
		return nil, reindex.ReindexConfig{}, false, true
	}
	defer req.Body.Close()

	var body reindex.ReindexConfig
	err = json.Unmarshal(reqBody, &body)
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
		return nil, reindex.ReindexConfig{}, false, true
	}

	// By default, wait_for_completion depends on size of index
	param := req.URL.Query().Get("wait_for_completion")
	if param == "" {
		// Get the size of currentIndex, if that is > IndexStoreSize (100MB - 100000000 Bytes)  then do async re-indexing.
		size, err := getIndexSize(req.Context(), sourceIndex)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Unable to get the size of "+sourceIndex, http.StatusBadRequest)
			return nil, reindex.ReindexConfig{}, false, true
		}
		if size > reindex.IndexStoreSize {
			param = "false"
		} else {
			param = "true"
		}
	}
	waitForCompletion, err := strconv.ParseBool(param)
	if err != nil {
		log.Errorln(logTag, ":", err)
		util.WriteBackError(w, err.Error(), http.StatusBadRequest)
		return nil, reindex.ReindexConfig{}, false, true
	}
	return err, body, waitForCompletion, false
}
