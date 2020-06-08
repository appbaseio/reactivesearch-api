package elasticsearch

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/util"
	es7 "github.com/olivere/elastic/v7"
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
		esClient := util.GetClient7()
		fmt.Println("options => ", es7.PerformRequestOptions{
			Method:  r.Method,
			Path:    r.URL.Path,
			Body:    r.Body,
			Headers: r.Header,
			Params:  r.URL.Query(),
		})
		response, err := esClient.PerformRequest(ctx, es7.PerformRequestOptions{
			Method:  r.Method,
			Path:    r.URL.Path,
			Params:  r.URL.Query(),
			Body:    r.Body,
			Headers: r.Header,
		})

		if err != nil {
			fmt.Println("=>", string(response.Body))
			log.Errorln(logTag, ": error fetching response for", r.URL.Path, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Copy the headers
		for k, v := range response.Header {
			if k != "Content-Length" {
				w.Header().Set(k, v[0])
			}
		}
		w.Header().Set("X-Origin", "ES")

		// Copy the status code
		w.WriteHeader(response.StatusCode)

		// Copy the body
		fmt.Println("res", string(response.Body))
		io.Copy(w, bytes.NewReader(response.Body))
	}
}
