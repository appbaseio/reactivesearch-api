package elasticsearch

import (
	"io"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

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
		// Forward the request to elasticsearch
		// esClient := util.GetClient7()

		// remove content-type header from r.Headers as that is internally managed my oliver
		// and can give following error if passed `{"error":{"code":500,"message":"elastic: Error 400 (Bad Request): java.lang.IllegalArgumentException: only one Content-Type header should be provided [type=content_type_header_exception]","status":"Internal Server Error"}}`
		headers := http.Header{}
		for k, v := range r.Header {
			if k != "Content-Type" {
				headers.Set(k, v[0])
			}
		}

		params := r.URL.Query()
		formatParam := params.Get("format")
		// need to add check for `strings.Contains(r.URL.Path, "_cat")` because
		// ACL for root route `/` is also `Cat`.
		if *reqACL == acl.Cat && strings.Contains(r.URL.Path, "_cat") && formatParam == "" {
			params.Add("format", "text")
		}

		// requestOptions := es7.PerformRequestOptions{
		// 	Method:  r.Method,
		// 	Path:    r.URL.Path,
		// 	Params:  params,
		// 	Headers: headers,
		// }

		// convert body to string string as oliver Perform request can accept io.Reader, String, interface
		// body, err := ioutil.ReadAll(r.Body)
		// if len(body) > 0 {
		// 	requestOptions.Body = string(body)
		// }
		req, _ := http.NewRequest(r.Method, util.GetESURL()+r.URL.Path, r.Body)
		req.Header.Add("Content-Type", "application/json")
		req.Header.Add("cache-control", "no-cache")
		start := time.Now()
		response, err := http.DefaultClient.Do(req)
		log.Println("TIME TAKEN BY ES:", time.Since(start))
		if err != nil {
			log.Errorln(logTag, ": error while sending request :", r.URL.Path, err)
			util.WriteBackError(w, err.Error(), response.StatusCode)
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
		w.Header().Set("X-Origin", "ES")
		// Copy the status code
		w.WriteHeader(response.StatusCode)

		// Copy the body
		io.Copy(w, response.Body)
		if err != nil {
			log.Errorln(logTag, ": error fetching response for", r.URL.Path, err)
			util.WriteBackError(w, err.Error(), response.StatusCode)
			return
		}
	}
}
