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

		// Create a new set of headers with only the allowed ones
		headers := http.Header{}

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

		// Convert body to string as oliver Perform request can accept io.Reader, String, interface
		body, err := ioutil.ReadAll(r.Body)
		if len(body) > 0 {
			requestOptions.Body = string(body)
		}

		start := time.Now()
		response, err := util.GetClient7().PerformRequest(ctx, requestOptions)
		log.Println(fmt.Sprintf("TIME TAKEN BY ES: %dms", time.Since(start).Milliseconds()))
		if err != nil {
			// Log the general error
			log.Errorln(logTag, ": error while sending request :", r.URL.Path, err)

			// Check if response is not nil to read the error details
			if response != nil {
				var errMsg string
				if response.Body != nil {
					// Read the response body
					bodyBytes, readErr := ioutil.ReadAll(bytes.NewReader(response.Body))
					if readErr != nil {
						errMsg = fmt.Sprintf("error reading response body: %v", readErr)
					} else {
						// Convert the body to a string and include it in the error message
						errMsg = fmt.Sprintf("error response from ES: %s", string(bodyBytes))
					}
				} else {
					errMsg = "response body is nil"
				}

				// Log the detailed error message
				log.Errorln(logTag, ": detailed error response: ", errMsg)

				// Write back the error with telemetry
				telemetry.WriteBackErrorWithTelemetry(r, w, errMsg, response.StatusCode)
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
		if _, copyErr := io.Copy(w, bytes.NewReader(response.Body)); copyErr != nil {
			log.Errorln(logTag, ": error writing response for", r.URL.Path, copyErr)
			telemetry.WriteBackErrorWithTelemetry(r, w, copyErr.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Origin", "reactivesearch.io")
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

func (es *elasticsearch) pingES() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, code, err := util.GetClient7().Ping(util.GetESURL()).Do(context.Background())
		if err != nil {
			log.Errorln(logTag, ": error fetching ES cluster health", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, err.Error(), http.StatusInternalServerError)
			return
		}
		responseInBytes, err := json.Marshal(result)
		if err != nil {
			log.Errorln(logTag, ": error while marshalling the ping result", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
		}
		util.WriteBackRaw(w, responseInBytes, code)
	}
}
