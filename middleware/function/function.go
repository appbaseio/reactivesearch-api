package function

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/antonmedv/expr"
	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/index"
	"github.com/appbaseio/arc/util"
	log "github.com/sirupsen/logrus"
)

// parse splits the comma separated key-value pairs (k1=v1, k2=v3) present in the header.
func parse(header string) []map[string]string {
	var m []map[string]string
	tokens := strings.Split(header, ",")
	for _, token := range tokens {
		values := strings.Split(token, "=")
		if len(values) == 2 {
			m = append(m, map[string]string{
				"key":   strings.TrimSpace(values[0]),
				"value": strings.TrimSpace(values[1]),
			})
		}
	}
	return m
}

func invokeFunction(functionDetails ESFunctionDoc, body InvokeFunctionBody) ([]byte, *http.Response, error) {
	log.Println("Invoking function")
	marshalledbody, err := json.Marshal(body)
	if err != nil {
		log.Errorln(LogTag, ":", err)
		return nil, nil, err
	}
	bodyRes, httpRes, err2 := MakeOpenFaasRequest("/function/"+functionDetails.Function.Service, http.MethodPost, marshalledbody)
	if err2 != nil {
		log.Errorln(LogTag, ":", err)
		return nil, nil, err2
	}
	return bodyRes, httpRes, err2
}

func getEnvironments(req *http.Request) (*TriggerEnvironments, error) {
	ctx := req.Context()
	reqACL, err := acl.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	reqCategory, err := category.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	indices, err := index.FromContext(req.Context())
	if err != nil {
		return nil, err
	}

	query := req.Header.Get(XSearchQuery)
	searchFilters := parse(req.Header.Get(XSearchFilters))

	return &TriggerEnvironments{
		ACL:      reqACL.String(),
		Category: reqCategory.String(),
		Query:    query,
		Filter:   searchFilters,
		Now:      time.Now().Unix(),
		Index:    indices,
	}, nil
}

func getRequestHeaders(req *http.Request) *map[string]string {
	headers := make(map[string]string)
	for header := range req.Header {
		headers[header] = req.Header.Get(header)
	}
	return &headers
}

func getResponseHeaders(res *http.Response) *map[string]string {
	headers := make(map[string]string)
	for header := range res.Header {
		headers[header] = res.Header.Get(header)
	}
	return &headers
}

func getParsedBody(req *http.Request) (*map[string]interface{}, error) {
	reqBody, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if len(reqBody) == 0 {
		return nil, nil
	}
	var parsedBody map[string]interface{}
	err2 := json.Unmarshal(reqBody, &parsedBody)
	if err2 != nil {
		return nil, err2
	}
	return &parsedBody, nil
}

func getRequestBody(req *http.Request) (*InvokeRequest, error) {
	body, err := getParsedBody(req)
	if err != nil {
		return nil, err
	}
	return &InvokeRequest{
		URL:     req.URL.String(),
		Method:  req.Method,
		Headers: getRequestHeaders(req),
		Body:    body,
	}, nil
}

func validateFilter(req *http.Request, functionDetails ESFunctionDoc) (bool, error) {
	if functionDetails.Trigger != nil && functionDetails.Trigger.Type != nil && functionDetails.Trigger.Type.String() == Filter.String() {
		// Apply the filter logic
		environments, err := getEnvironments(req)
		if err != nil {
			return false, err
		}
		program, err := expr.Compile(functionDetails.Trigger.Expression, expr.Env(environments), expr.AsBool())
		if err != nil {
			return false, err
		}

		output, err := expr.Run(program, environments)
		if err != nil {
			return false, err
		}
		return output.(bool), nil
	}
	return true, nil
}

func before(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		for _, functionDetails := range GetFunctionsFromCache() {
			if !functionDetails.Enabled && functionDetails.Trigger != nil && !functionDetails.Trigger.ExecuteBefore {
				// Ignore if execute before is set to false
				continue
			}
			ok, err := validateFilter(req, functionDetails)
			if err != nil {
				msg := "Error encountered while evaluating the expression." + err.Error()
				log.Errorln(LogTag, ":", err)
				util.WriteBackError(w, msg, http.StatusInternalServerError)
				return
			}
			if ok {
				environments, err := getEnvironments(req)
				if err != nil {
					msg := "Error encountered while evaluating the expression." + err.Error()
					log.Errorln(LogTag, ":", err)
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				requestBody, err := getRequestBody(req)
				if err != nil {
					log.Errorln(LogTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				fmt.Println("Invoking Before", functionDetails)
				returnedBody, httpRes, err := invokeFunction(functionDetails, InvokeFunctionBody{
					ExtraRequestPayload: functionDetails.ExtraRequestPayload,
					Environments:        *environments,
					Request:             requestBody,
				})
				if err != nil {
					log.Errorln(LogTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if httpRes.StatusCode != http.StatusOK {
					// Handle failure, return response
					util.WriteBackError(w, string(returnedBody), httpRes.StatusCode)
					return
				}
				// Handle success
				var modifiedRequest InvokeFunctionBody
				err2 := json.Unmarshal(returnedBody, &modifiedRequest)
				if err2 != nil {
					log.Errorln(LogTag, ":", err2)
					msg := "Unable to unmarshal request body returned by function " + functionDetails.Function.Service
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				// Apply function modification on request
				if modifiedRequest.Request != nil {
					req.RequestURI = modifiedRequest.Request.URL
					req.Method = modifiedRequest.Request.Method
					if modifiedRequest.Request.Headers != nil {
						for key, header := range *modifiedRequest.Request.Headers {
							req.Header.Set(key, header)
						}
					}
					if modifiedRequest.Request.Body != nil {
						body, err := json.Marshal(modifiedRequest.Request.Body)
						if err != nil {
							log.Errorln(LogTag, ":", err)
							util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
							return
						}
						req.Body = ioutil.NopCloser(bytes.NewBufferString(string(body)))
					}
				}
			}
		}
		h(w, req)
	})
}

func after(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Println("AFTER MIDDLEWARE")
		resp := httptest.NewRecorder()
		h(resp, req)

		result := resp.Result()
		body, err := ioutil.ReadAll(result.Body)
		if err != nil {
			log.Errorln(LogTag, ":", err)
			util.WriteBackError(w, "error reading response body", http.StatusInternalServerError)
			return
		}

		var responseBody map[string]interface{}
		err = json.Unmarshal(body, &responseBody)
		if err != nil {
			log.Errorln(LogTag, ":", err)
			util.WriteBackError(w, "error un-marshaling search result", http.StatusInternalServerError)
			return
		}

		invokeResponse := &InvokeResponse{
			Body:    &responseBody,
			Headers: getResponseHeaders(result),
			Status:  result.Status,
		}

		for _, functionDetails := range GetFunctionsFromCache() {
			fmt.Println("AFTER MIDDLEWARE GOT CALLED", functionDetails)
			if functionDetails.Enabled && functionDetails.Trigger != nil && !functionDetails.Trigger.ExecuteBefore {
				fmt.Println("FUNCTION IS")
				// invoke when execute before is false
				ok, err := validateFilter(req, functionDetails)
				if err != nil {
					msg := "Error encountered while evaluating the expression." + err.Error()
					log.Errorln(LogTag, ":", err)
					util.WriteBackError(w, msg, http.StatusInternalServerError)
					return
				}
				if ok {
					environments, err := getEnvironments(req)
					if err != nil {
						msg := "Error encountered while evaluating the expression." + err.Error()
						log.Errorln(LogTag, ":", err)
						util.WriteBackError(w, msg, http.StatusInternalServerError)
						return
					}
					fmt.Println("INVOKING AFTER")
					returnedBody, httpRes, err := invokeFunction(functionDetails, InvokeFunctionBody{
						ExtraRequestPayload: functionDetails.ExtraRequestPayload,
						Environments:        *environments,
						Response:            invokeResponse,
					})
					if err != nil {
						log.Errorln(LogTag, ":", err)
						util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
						return
					}
					// Handle failure, return response
					if httpRes.StatusCode != http.StatusOK {
						util.WriteBackError(w, string(returnedBody), httpRes.StatusCode)
						return
					}
					// Handle success
					var modifiedRequest InvokeFunctionBody
					err2 := json.Unmarshal(returnedBody, &modifiedRequest)
					if err2 != nil {
						log.Errorln(LogTag, ":", err2)
						msg := "Unable to unmarshal response body returned by function " + functionDetails.Function.Service
						util.WriteBackError(w, msg, http.StatusInternalServerError)
						return
					}
					// Apply function modification on response
					if modifiedRequest.Response != nil {
						invokeResponse.Body = modifiedRequest.Response.Body
						invokeResponse.Status = modifiedRequest.Response.Status
						if modifiedRequest.Response.Headers != nil {
							for key, header := range *modifiedRequest.Response.Headers {
								w.Header().Set(key, header)
							}
						}
					}
				}
			} else {
				continue
			}
		}
		raw, err := json.Marshal(invokeResponse.Body)
		if err != nil {
			log.Errorln(LogTag, ":", err)
			util.WriteBackError(w, "error marshaling search result", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	})
}

// Before middleware invokes the functions which are defined to execute before search request
func Before() middleware.Middleware {
	return before
}

// After middleware invokes the functions which are defined to execute before search request
func After() middleware.Middleware {
	return after
}
