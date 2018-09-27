package es

import (
	"io"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/util"
)

func (es *ES) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		c := ctx.Value(category.CtxKey)
		if c != nil {
			log.Printf("%s: acl=%s\n", logTag, c)
		}
		o := ctx.Value(op.CtxKey)
		if o != nil {
			log.Printf("%s: operation=%s\n", logTag, o)
		}

		client := httpClient()
		response, err := client.Do(r)
		if err != nil {
			log.Printf("%s: error fetching response for %s: %v\n", logTag, r.URL.Path, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer response.Body.Close()

		// Copy the headers
		for k, v := range response.Header {
			w.Header()[k] = v
		}

		// Copy the status code
		w.WriteHeader(response.StatusCode)

		// Copy the body
		io.Copy(w, response.Body)
	}
}
