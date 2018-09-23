package es

import (
	"io"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/util"
)

func redirectHandler(w http.ResponseWriter, r *http.Request) {
	client := httpClient()
	response, err := client.Do(r)
	if err != nil {
		log.Printf("[ERROR]: error fetching response for %s: %v", r.URL.Path, err)
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
