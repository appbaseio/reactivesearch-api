package es

import (
	"io"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/util"
)

func (es *ES) redirectHandler(w http.ResponseWriter, r *http.Request) {
	client := httpClient()
	response, err := client.Do(r)
	if err != nil {
		log.Printf("%s: error fetching response for %s: %v", logTag, r.URL.Path, err)
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
