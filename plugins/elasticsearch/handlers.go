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

	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
)

func (es *elasticsearch) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "error classifying request acl", http.StatusInternalServerError)
			return
		}

		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "error classifying request category", http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "error classifying request op", http.StatusInternalServerError)
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
		//
		// Skip adding the Accept header since it is passed by default as */* and Elastic doesn't like that and ends up throwing
		// Invalid media-type value on header [Accept] [type=media_type_header_exception]
		headers := http.Header{}
		for k := range r.Header {
			if k == "Content-Type" || k == "Authorization" || k == "Accept" {
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
				telemetry.WriteBackErrorWithTelemetry(r, w, err.Error(), response.StatusCode)
				return
			}
			telemetry.WriteBackErrorWithTelemetry(r, w, err.Error(), http.StatusInternalServerError)
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
			telemetry.WriteBackErrorWithTelemetry(r, w, err.Error(), response.StatusCode)
			return
		}
	}
}

func (es *elasticsearch) healthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, code, err := util.GetClient7().Ping(util.GetESURL()).Do(context.Background())
		if err != nil {
			log.Errorln(logTag, ": error fetching cluster health", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, []byte{}, code)
	}
}

// dryHealthCheck will return a dummy response everytime it's
// called. This method can be used to check if the server is stuck
// and whether or not a restart is necessary.
func dryHealthCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		response := map[string]interface{}{
			"health": "ok",
		}

		// Marshal the response
		//
		// NOTE: No need to check error since response is manually created
		// in the above lines
		responseInBytes, _ := json.Marshal(response)
		util.WriteBackRaw(w, responseInBytes, http.StatusOK)
	}
}
