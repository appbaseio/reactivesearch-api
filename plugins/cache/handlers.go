package cache

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

func (c *Cache) savePreferencesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Get saved preferences
			response, err := c.es.getPreferences(req.Context())
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusNotFound)
				return
			}
			// Update local variable
			setCachePreferences(response)
			util.WriteBackMessage(w, "Cache preferences saved successfully", http.StatusOK)
			return
		}
		defer req.Body.Close()
		var cacheConfig CacheConfig
		err := json.NewDecoder(req.Body).Decode(&cacheConfig)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, fmt.Sprintf("Can't parse request body: %v", err), http.StatusBadRequest)
			return
		}
		// Validate cache preferences
		err2 := validateCacheConfig(cacheConfig)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			util.WriteBackError(w, err2.Error(), http.StatusBadRequest)
			return
		}
		err3 := c.es.savePreferences(req.Context(), cacheConfig)
		if err3 != nil {
			log.Errorln(logTag, ":", err3)
			util.WriteBackError(w, err3.Error(), http.StatusInternalServerError)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Invoke ACCAPI
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPost,
				URL:    "/_cache/preferences",
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating cache preferences")
				bodyBytes, err := io.ReadAll(res.Body)
				log.Errorln("error is: ", string(bodyBytes))
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update local variable
			setCachePreferences(cacheConfig)
		}
		util.WriteBackMessage(w, "Cache preferences saved successfully", http.StatusOK)
	}
}

func (c *Cache) clearCacheHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Clear cache
			c.es.clearCache()
			util.WriteBackMessage(w, "Cached requests deleted successfully", http.StatusOK)
			return
		}
		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause insconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Invoke ACCAPI
			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPost,
				URL:    "/_cache/evict",
			})
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered evicting cache")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Clear cache
			c.es.clearCache()
		}
		util.WriteBackMessage(w, "Cached requests deleted successfully", http.StatusOK)
	}
}

func (c *Cache) getPreferencesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		response, err := c.es.getPreferences(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusNotFound)
			return
		}
		res, err := json.Marshal(response)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, res, http.StatusOK)
	}
}
