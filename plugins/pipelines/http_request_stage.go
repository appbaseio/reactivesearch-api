package pipelines

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/appbaseio-confidential/reactivesearch/plugins/rules"
	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

type HTTPRequestInput struct {
	Method  *string            `json:"method,omitempty" jsonschema:"title=Request Method" jsonschema_description:"Http request method, for e.g, 'POST'. The default value is the pipeline request method."`
	URL     *string            `json:"url,omitempty" jsonschema:"title=Request URL,required" jsonschema_description:"Request URL, for e.g, 'https://appbase-demo-ansible-abxiydt-arc.searchbase.io/good-books-ds/_search'."`
	Params  *map[string]string `json:"params,omitempty" jsonschema:"title=Query params" jsonschema_description:"Request query params, for e.g, '{ format: 'JSON' }'"`
	Headers *map[string]string `json:"headers,omitempty" jsonschema:"title=Request Headers" jsonschema_description:"Request headers, for e.g, '{ Content-Type: 'application/json' }'"`
	Body    *string            `json:"body,omitempty" jsonschema:"title=Body" jsonschema_description:"Request body in string format, e.g, {\"query\":{\"match_all\":{}}}."`
}

func GetHTTPRequestInputSchema() map[string]interface{} {
	schema := GetReflactor().Reflect(&HTTPRequestInput{})
	inputsSchema, _ := schema.MarshalJSON()
	var schemaInBytes map[string]interface{}
	json.Unmarshal(inputsSchema, &schemaInBytes)
	return schemaInBytes
}

// Returns the final envs to be used
func getHTTPInputs(
	scriptEnvs HTTPRequestInput,
	stageInputs *string) (HTTPRequestInput, error) {
	finalEnvs := scriptEnvs
	if stageInputs == nil {
		return finalEnvs, nil
	}

	var parsedESInputs HTTPRequestInput
	log.Debug(logTag, ": inputs: ", string(*stageInputs))
	err2 := json.Unmarshal([]byte(*stageInputs), &parsedESInputs)
	if err2 != nil {
		return finalEnvs, err2
	}
	// merge inputs
	if parsedESInputs.Body != nil {
		finalEnvs.Body = parsedESInputs.Body
	}
	if parsedESInputs.URL != nil {
		httpURL := *parsedESInputs.URL
		if strings.Contains(httpURL, "@") {
			splitIndex := strings.LastIndex(httpURL, "@")
			protocolWithCredentials := strings.Split(httpURL[0:splitIndex], "://")
			if len(protocolWithCredentials) > 1 {
				credentials := protocolWithCredentials[1]
				protocol := protocolWithCredentials[0]
				host := httpURL[splitIndex+1:]

				credentialSeparator := strings.Index(credentials, ":")
				username := credentials[0:credentialSeparator]
				password := credentials[credentialSeparator+1:]
				httpURL = protocol + "://" + url.PathEscape(username) + ":" + url.PathEscape(password) + "@" + host
			}
		}
		finalEnvs.URL = &httpURL
	}
	if parsedESInputs.Method != nil {
		finalEnvs.Method = parsedESInputs.Method
	}
	if parsedESInputs.Headers != nil {
		finalEnvs.Headers = parsedESInputs.Headers
	}
	if parsedESInputs.Params != nil {
		finalEnvs.Params = parsedESInputs.Params
	}
	return finalEnvs, nil
}

func executeHTTPRequestStage(
	stage ESPipelineStage,
	parsedInputs *string,
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

	// default method
	method := http.MethodGet
	var requestBody *string
	paramsMap := make(map[string]string)
	headersMap := make(map[string]string)

	// only sync can read the request from context
	// Only sync stage can modify the final response
	if !async {
		// Don't forward Authorization and Accept headers
		headers := http.Header{}
		for k := range scriptContext.Request.Headers {
			if k == "Authorization" || k == "Accept" {
				continue
			}
			headers.Set(k, scriptContext.Request.Headers[k])
		}

		// extract request method
		reqMethod, ok := scriptEnvs["method"].(string)
		if ok {
			method = reqMethod
		}

		paramsAsMap, ok := scriptEnvs["urlValues"].(map[string]interface{})
		if ok {
			for k, v := range paramsAsMap {
				paramValue, ok := v.(string)
				if ok {
					paramsMap[k] = paramValue
				}
			}
		}

		for k := range headers {
			headersMap[k] = headers.Get(k)
		}

		requestBody = &scriptContext.Request.Body
	}

	inputs, err := getHTTPInputs(HTTPRequestInput{
		Method:  &method,
		Body:    requestBody,
		Params:  &paramsMap,
		Headers: &headersMap,
	}, parsedInputs)
	if err != nil {
		errorMsg := fmt.Errorf("error reading inputs for stage: "+*id+", %s", err.Error())
		log.Errorln(logTag, errorMsg)
		return nil, false, &Error{
			Err:  errorMsg,
			Code: http.StatusBadRequest,
		}
	}

	// URL must be present
	if inputs.URL == nil || *inputs.URL == "" {
		return nil, false, &Error{
			Err:  fmt.Errorf("The 'url' property must be present for stage: " + *id),
			Code: http.StatusBadRequest,
		}
	}

	// Extract params from envs
	var params = make(url.Values)
	if inputs.Params != nil {
		for k, v := range *inputs.Params {
			params.Set(k, v)
		}
	}
	// apply input headers
	var requestHeader = make(http.Header)
	if inputs.Headers != nil {
		for k, v := range *inputs.Headers {
			requestHeader.Set(k, v)
		}
	}

	var httpRequestBody io.Reader
	if inputs.Body != nil {
		httpRequestBody = bytes.NewBuffer([]byte(*inputs.Body))
	}

	httpRequest, err := http.NewRequest(*inputs.Method, *inputs.URL, httpRequestBody)
	if err != nil {
		log.Errorln(logTag, ":", err.Error())
		return nil, false, &Error{
			Err: err,
		}
	}

	// apply params
	q := httpRequest.URL.Query()
	for k := range params {
		q.Set(k, params.Get(k))
	}
	httpRequest.URL.RawQuery = q.Encode()

	// Set default content-type as application/json
	httpRequest.Header.Set("Content-Type", "application/json")
	// apply headers
	for k := range requestHeader {
		httpRequest.Header.Set(k, requestHeader.Get(k))
	}

	// perform Request
	log.Debugln("Pipeline Elasticsearch: REQUEST METHOD", httpRequest.Method)
	log.Debugln("Pipeline Elasticsearch: REQUEST URL", httpRequest.URL.String())
	if inputs.Body != nil {
		log.Debugln("Pipeline Elasticsearch: REQUEST BODY", *inputs.Body)
	}
	log.Debugln("Pipeline Elasticsearch: REQUEST HEADERS", httpRequest.Header)
	response, err := util.HTTPClient().Do(httpRequest)
	if err != nil {
		log.Errorln(logTag, ": error while sending request :", *inputs.URL, err)
		if response != nil {
			log.Errorln(logTag, ":", err.Error())
			return nil, false, &Error{
				Err:  err,
				Code: response.StatusCode,
			}
		}
		log.Errorln(logTag, ":", err.Error())
		return nil, false, &Error{
			Err: err,
		}
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		log.Errorln(logTag, ":", err.Error())
		return nil, false, &Error{
			Err: err,
		}
	}
	log.Debugln("Pipeline Elasticsearch: RESPONSE", string(responseBody))
	// Set response to script context
	// set headers
	if response.Header != nil {
		for k := range response.Header {
			if k != "Content-Length" {
				scriptContext.Response.Headers[k] = response.Header.Get(k)
			}
		}
	}
	var output interface{}
	shouldStopExecution := false
	if async {
		// write output to a top-level variable
		output = map[string]interface{}{
			*id: string(responseBody),
		}
	} else {
		// set response code
		scriptContext.Response.Code = response.StatusCode
		scriptContext.Response.Body = string(responseBody)
		scriptContext.Response.Headers["X-Origin"] = "reactivesearch.io"

		// Set request values
		scriptContext.Request.Body = *inputs.Body
		scriptContext.Request.URL = *inputs.URL
		scriptContext.Request.Method = *inputs.Method

		requestHeaders := make(map[string]string)
		for key, value := range httpRequest.Header {
			requestHeaders[key] = strings.Join(value, ", ")
		}
		scriptContext.Request.Headers = requestHeaders

		output = scriptContext

		// stop execution for >= 4xx response codes
		if scriptContext.Response.Code >= 400 {
			shouldStopExecution = true
		}
	}
	contextInBytes, err := json.Marshal(output)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return nil, false, &Error{
			Err: err,
		}
	}
	return contextInBytes, shouldStopExecution, nil
}
