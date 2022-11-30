package util

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	log "github.com/sirupsen/logrus"
)

var totalUsage int

func addToUsage(usage int) {
	totalUsage += usage
}

// Method to clear reported usage
func ClearUsage() {
	totalUsage = 0
}

// Returns the total usage
func GetDataUsage() int {
	return totalUsage
}

func RecordUsageMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println("total request usage", GetDataUsage())
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, r)
		// Copy the response to writer
		for k, v := range resp.Header() {
			w.Header()[k] = v
		}
		go func(request *http.Request) {
			if request.Body != nil {
				// Read request and update usage
				requestBody, err := ioutil.ReadAll(request.Body)
				if err != nil {
					log.Errorln(" error while reading body from request, ", err)
				} else {
					requestUsage := len(requestBody)
					// Add usage
					log.Infoln("request usage reported: ", requestUsage)
					addToUsage(requestUsage)
				}
			}
		}(r)
		w.WriteHeader(resp.Code)
		response := resp.Body.Bytes()
		go func(responseInBytes []byte) {
			// Read response and update usage
			responseUsage := len(responseInBytes)
			// Add usage
			log.Infoln("response usage reported: ", responseUsage)
			// Add usage
			addToUsage(responseUsage)
		}(response)
		w.Write(response)
	})
}
