package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	es7 "github.com/olivere/elastic/v7"

	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/util"
)

func (es *elasticsearch) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "error classifying request acl", http.StatusInternalServerError)
			return
		}

		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "error classifying request category", http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "error classifying request op", http.StatusInternalServerError)
			return
		}
		log.Println(logTag, ": category=", *reqCategory, ", acl=", *reqACL, ", op=", *reqOp)
		// disable gzip compression
		encoding := r.Header.Get("Accept-Encoding")
		if encoding != "" {
			r.Header.Set("Accept-Encoding", "identity")
		}
		// Forward the request to elasticsearch
		// remove content-type header from r.Headers as that is internally managed my oliver
		// and can give following error if passed `{"error":{"code":500,"message":"elastic: Error 400 (Bad Request): java.lang.IllegalArgumentException: only one Content-Type header should be provided [type=content_type_header_exception]","status":"Internal Server Error"}}`
		headers := http.Header{}
		for k := range r.Header {
			if k == "Content-Type" || k == "Authorization" {
				continue
			}
			headers.Set(k, r.Header.Get(k))
		}

		params := r.URL.Query()
		formatParam := params.Get("format")
		// need to add check for `strings.Contains(r.URL.Path, "_cat")` because
		// ACL for root route `/` is also `Cat`.
		if *reqACL == acl.Cat && strings.Contains(r.URL.Path, "_cat") && formatParam == "" {
			params.Add("format", "text")
		}

		requestOptions := es7.PerformRequestOptions{
			Method:  r.Method,
			Path:    r.URL.Path,
			Params:  params,
			Headers: headers,
		}

		// convert body to string string as oliver Perform request can accept io.Reader, String, interface
		body, err := ioutil.ReadAll(r.Body)
		if len(body) > 0 {
			requestOptions.Body = string(body)
		}
		start := time.Now()
		response, err := util.GetClient7().PerformRequest(ctx, requestOptions)
		log.Println(fmt.Sprintf("TIME TAKEN BY ES: %dms", time.Since(start).Milliseconds()))
		if err != nil {
			log.Errorln(logTag, ": error while sending request :", r.URL.Path, err)
			if response != nil {
				util.WriteBackError(w, err.Error(), response.StatusCode)
				return
			}
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Copy the headers
		if response.Header != nil {
			for k, v := range response.Header {
				if k != "Content-Length" {
					w.Header().Set(k, v[0])
				}
			}
		}
		w.WriteHeader(response.StatusCode)
		// Copy the body
		io.Copy(w, bytes.NewReader(response.Body))
		w.Header().Set("X-Origin", "appbase.io")
		if err != nil {
			log.Errorln(logTag, ": error fetching response for", r.URL.Path, err)
			util.WriteBackError(w, err.Error(), response.StatusCode)
			return
		}
	}
}

func (es *elasticsearch) healthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, code, err := util.GetClient7().Ping(util.GetESURL()).Do(context.Background())
		if err != nil {
			log.Errorln(logTag, ": error fetching cluster health", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
		}
		responseInBytes, err := json.Marshal(result)
		if err != nil {
			log.Errorln(logTag, ": error while marshalling the ping result", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
		}
		var response map[string]interface{}
		err2 := json.Unmarshal(responseInBytes, &response)
		if err2 != nil {
			log.Errorln(logTag, ": error while un-marshalling the response", err2)
			util.WriteBackError(w, err2.Error(), http.StatusInternalServerError)
		}
		response["appbase_version"] = util.Version
		finalResponseInBytes, err := json.Marshal(response)
		if err != nil {
			log.Errorln(logTag, ": error while marshalling the response", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
		}
		util.WriteBackRaw(w, finalResponseInBytes, code)
	}
}
