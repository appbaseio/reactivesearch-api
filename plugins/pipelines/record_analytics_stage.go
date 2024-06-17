package pipelines

import (
	"encoding/json"

	"github.com/appbaseio-confidential/reactivesearch/model/category"
	"github.com/appbaseio-confidential/reactivesearch/plugins/analytics"
	"github.com/appbaseio-confidential/reactivesearch/plugins/querytranslate"
	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	log "github.com/sirupsen/logrus"
)

func executeRecordAnalyticsStage(scriptContextInBytes []byte, scriptEnvs map[string]interface{}, rsAPIRequest *ReactiveSearchQueryContext) ([]byte, *Error) {
	{
		// apply headers from script context
		var scriptContext rules.ScriptContext
		err2 := json.Unmarshal(scriptContextInBytes, &scriptContext)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			return nil, &Error{
				Err: err2,
			}
		}
		// read RS API from channel
		rsQuery := rsAPIRequest.Get()

		var requestToRecordAnalytics querytranslate.RSQuery
		if rsQuery != nil {
			requestToRecordAnalytics = *rsQuery
		} else {
			err := json.Unmarshal([]byte(scriptContext.Request.Body), &requestToRecordAnalytics)
			if err != nil {
				log.Errorln(logTag, ":", err)
				return nil, &Error{
					Err: err,
				}
			}
		}
		reqCategory, ok := scriptEnvs["category"].(string)
		if ok && reqCategory == category.ReactiveSearch.String() {
			queryId, response := analytics.Instance().RecordAnalytics(scriptContext.Request, &requestToRecordAnalytics, scriptContext.Response, getIndicesFromEnvs(scriptEnvs))
			if queryId != nil {
				// write query id in headers
				scriptContext.Response.Headers = response.Headers
				scriptContext.Response.Body = response.Body
				// write response with query id in settings
				contextInBytes, err := json.Marshal(scriptContext)
				if err != nil {
					log.Errorln(logTag, ":", err)
					return nil, &Error{
						Err: err,
					}
				}
				return contextInBytes, nil
			}
		}
	}
	return scriptContextInBytes, nil
}
