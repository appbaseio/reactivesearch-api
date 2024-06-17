package pipelines

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/plugins/cache"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	log "github.com/sirupsen/logrus"
)

func executeRecordCacheStage(
	originalRequest []byte,
	scriptEnvs map[string]interface{},
	scriptContextInBytes []byte) *Error {
	var reqURL string
	// Extract path from envs
	path, ok := scriptEnvs["path"].(string)
	if ok {
		reqURL = path
	}
	// apply headers from script context
	var scriptContext rules.ScriptContext
	err := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return &Error{
			Err: err,
		}
	}
	// avoid re-recording for cache responses
	if scriptContext.Response.Headers[cache.CachedRequestHeader] == "true" {
		return nil
	}
	useCache := true
	// react cache query param
	paramsAsMap, ok := scriptEnvs["urlValues"].(map[string]interface{})
	if ok {
		for k, v := range paramsAsMap {
			paramValue, ok := v.(bool)
			if ok && k == XCacheHeader {
				useCache = paramValue
			}
		}
	}
	// read x-cache header
	for k, v := range scriptContext.Request.Headers {
		if k == XCacheHeader {
			boolValue, err := strconv.ParseBool(v)
			if err == nil {
				useCache = boolValue
			}
		}
	}
	uri := constructURL(reqURL, paramsAsMap)
	var rsAPIBody *querytranslate.RSQuery
	reqCategory, ok := scriptEnvs["category"].(string)
	if ok && reqCategory == category.ReactiveSearch.String() {
		// read useCache for RS API
		var rsAPIRequest querytranslate.RSQuery
		err := json.Unmarshal([]byte(originalRequest), &rsAPIRequest)
		if err == nil {
			if rsAPIRequest.Settings != nil && rsAPIRequest.Settings.UseCache != nil {
				useCache = *rsAPIRequest.Settings.UseCache
			}
			rsAPIBody = &rsAPIRequest
		}
	}
	// only record 200 success responses
	if scriptContext.Response.Code == http.StatusOK {
		if useCache {
			// record cache
			// TODO: Handle syncing cache for AIAnswer
			go cache.RecordRequest(uri, rsAPIBody, originalRequest, []byte(scriptContext.Response.Body))
		}
	}
	return nil
}
