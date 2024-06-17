package pipelines

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/analytics"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	log "github.com/sirupsen/logrus"
)

func executeRecordClickStage(
	stage ESPipelineStage,
	req *http.Request,
	globalScriptContext *GlobalScriptContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	id := getStageID(stage)

	scriptContextInBytes := globalScriptContext.Get()
	// apply headers from script context
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}

	var clickRequestBody analytics.ClickRecord
	err3 := json.Unmarshal([]byte(scriptContext.Request.Body), &clickRequestBody)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return nil, false, &Error{
			Err: err3,
		}
	}
	response := analytics.Instance().RecordClick(req, clickRequestBody)
	var output interface{}
	if async {
		// write output to a top-level variable
		output = map[string]interface{}{
			*id: response.Body,
		}
	} else {
		// set response code
		scriptContext.Response.Code = response.Code
		scriptContext.Response.Body = response.Body
		headers := scriptContext.Response.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		for k, v := range response.Headers {
			headers[k] = v
		}
		scriptContext.Response.Headers = headers
		output = scriptContext
	}
	contextInBytes, err := json.Marshal(output)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}

func executeRecordConversionStage(
	stage ESPipelineStage,
	req *http.Request,
	globalScriptContext *GlobalScriptContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	id := getStageID(stage)

	scriptContextInBytes := globalScriptContext.Get()
	// apply headers from script context
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}

	var conversionRequestBody analytics.ConversionRecord
	err3 := json.Unmarshal([]byte(scriptContext.Request.Body), &conversionRequestBody)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return nil, false, &Error{
			Err: err3,
		}
	}
	response := analytics.Instance().RecordConversion(req, conversionRequestBody)
	var output interface{}
	if async {
		// write output to a top-level variable
		output = map[string]interface{}{
			*id: response.Body,
		}
	} else {
		// set response code
		scriptContext.Response.Code = response.Code
		scriptContext.Response.Body = response.Body
		headers := scriptContext.Response.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		for k, v := range response.Headers {
			headers[k] = v
		}
		scriptContext.Response.Headers = headers
		output = scriptContext
	}
	contextInBytes, err := json.Marshal(output)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}

func executeRecordSaveSearch(
	stage ESPipelineStage,
	req *http.Request,
	globalScriptContext *GlobalScriptContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	id := getStageID(stage)

	scriptContextInBytes := globalScriptContext.Get()
	// apply headers from script context
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}

	var saveSearchBody analytics.SavedSearchRequest
	err3 := json.Unmarshal([]byte(scriptContext.Request.Body), &saveSearchBody)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return nil, false, &Error{
			Err: err3,
		}
	}
	response := analytics.Instance().RecordSavedSearch(req, saveSearchBody)
	var output interface{}
	if async {
		// write output to a top-level variable
		output = map[string]interface{}{
			*id: response.Body,
		}
	} else {
		// set response code
		scriptContext.Response.Code = response.Code
		scriptContext.Response.Body = response.Body
		headers := scriptContext.Response.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		for k, v := range response.Headers {
			headers[k] = v
		}
		scriptContext.Response.Headers = headers
		output = scriptContext
	}
	contextInBytes, err := json.Marshal(output)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}

func executeRecordFavoriteStage(
	stage ESPipelineStage,
	req *http.Request,
	globalScriptContext *GlobalScriptContext,
	scriptEnvs map[string]interface{},
	async bool,
	startTime *time.Time) ([]byte, bool, *Error) {
	id := getStageID(stage)

	scriptContextInBytes := globalScriptContext.Get()
	// apply headers from script context
	var scriptContext rules.ScriptContext
	err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return nil, false, &Error{
			Err: err2,
		}
	}

	var favoriteRequestBody analytics.FavoriteRequest
	err3 := json.Unmarshal([]byte(scriptContext.Request.Body), &favoriteRequestBody)
	if err3 != nil {
		log.Errorln(logTag, ":", err3)
		return nil, false, &Error{
			Err: err3,
		}
	}
	response := analytics.Instance().RecordFavorite(req, favoriteRequestBody)
	var output interface{}
	if async {
		// write output to a top-level variable
		output = map[string]interface{}{
			*id: response.Body,
		}
	} else {
		// set response code
		scriptContext.Response.Code = response.Code
		scriptContext.Response.Body = response.Body
		headers := scriptContext.Response.Headers
		if headers == nil {
			headers = make(map[string]string)
		}
		for k, v := range response.Headers {
			headers[k] = v
		}
		scriptContext.Response.Headers = headers
		output = scriptContext
	}
	contextInBytes, err := json.Marshal(output)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, false, nil
}
