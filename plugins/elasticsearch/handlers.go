package elasticsearch

import (
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/model/acl"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/util"
	"github.com/hashicorp/go-retryablehttp"
)

func (es *elasticsearch) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Error(logTag, ": ", err)
			util.WriteBackError(w, "error classifying request acl", http.StatusInternalServerError)
			return
		}

		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Error(logTag, ": ", err)
			util.WriteBackError(w, "error classifying request category", http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Error(logTag, ": ", err)
			util.WriteBackError(w, "error classifying request op", http.StatusInternalServerError)
			return
		}
		log.Printf(`%s: category="%s", acl="%s", op="%s"\n`, logTag, *reqCategory, *reqACL, *reqOp)
		// Forward the request to elasticsearch
		client := retryablehttp.NewClient()

		request, err := retryablehttp.FromRequest(r)
		if err != nil {
			log.Error(logTag, ": error while converting to retryable request for", r.URL.Path, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		response, err := client.Do(request)

		if err != nil {
			log.Error(logTag, ": error fetching response for", r.URL.Path, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		defer response.Body.Close()

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
		io.Copy(w, response.Body)
	}
}
