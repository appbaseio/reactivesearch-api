package elasticsearch

import (
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"io"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/util"
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
			util.WriteBackError(w, "error classifyig request category", http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, "error classifying request op", http.StatusInternalServerError)
			return
		}
		log.Printf(`%s: acl="%s", category="%s", op="%s"\n`, logTag, *reqCategory, *reqACL, *reqOp)

		// Forward the request to elasticsearch
		client := util.HTTPClient()
		response, err := client.Do(r)
		if err != nil {
			log.Printf("%s: error fetching response for %s: %v\n", logTag, r.URL.Path, err)
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
