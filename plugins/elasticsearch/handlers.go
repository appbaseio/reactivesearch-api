package elasticsearch

import (
	"io"
	"log"
	"net/http"
	"time"

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
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "error classifying request acl", http.StatusInternalServerError)
			return
		}

		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "error classifying request category", http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "error classifying request op", http.StatusInternalServerError)
			return
		}
		log.Printf(`%s: category="%s", acl="%s", op="%s"\n`, logTag, *reqCategory, *reqACL, *reqOp)

		// Forward the request to elasticsearch
		client := util.HTTPClient()

		var response *http.Response
		util.Retry(3, 100*time.Millisecond, func() bool {
			response, err = client.Do(r)
			if err != nil {
				log.Printf("%s: error fetching response for %s: %v\n", logTag, r.URL.Path, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return false
			}
			err = nil
			return true
		})

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
