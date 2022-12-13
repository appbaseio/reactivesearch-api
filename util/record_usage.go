package util

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	log "github.com/sirupsen/logrus"
)

// Domain to usage map
var totalUsage map[string]int

func addToUsage(domain string, usage int) {
	if _, ok := totalUsage[domain]; ok {
		totalUsage[domain] += usage
	} else {
		totalUsage[domain] = usage
	}
}

// Method to clear reported usage
func ClearUsage(domain string) {
	delete(totalUsage, domain)
}

// Returns the total usage
func GetDataUsageByDomain(domain string) int {
	if _, ok := totalUsage[domain]; ok {
		return totalUsage[domain]
	}
	return 0
}

func RecordUsageMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domainInfo, err := domain.FromContext(r.Context())
		if err != nil {
			log.Errorln("error while reading domain from context")
			WriteBackError(w, "Please make sure that you're using a valid domain. If the issue persists please contact support@appbase.io with your domain or registered e-mail address.", http.StatusBadRequest)
			return
		}
		tenantId := GetTenantForDomain(domainInfo.Raw)
		log.Println("total request usage", GetDataUsageByDomain(tenantId))
		dumpRequest, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Errorln(err.Error())
			h.ServeHTTP(w, r)
			return
		}
		resp := httptest.NewRecorder()
		h.ServeHTTP(resp, r)
		// Copy the response to writer
		for k, v := range resp.Header() {
			w.Header()[k] = v
		}
		go func(requestBody []byte, tenantId string) {
			if requestBody != nil {
				requestUsage := len(requestBody)
				// Add usage
				log.Infoln("request usage reported: ", requestUsage)
				addToUsage(tenantId, requestUsage)

			}
		}(dumpRequest, tenantId)
		w.WriteHeader(resp.Code)
		response := resp.Body.Bytes()
		go func(responseInBytes []byte, tenantId string) {
			// Read response and update usage
			responseUsage := len(responseInBytes)
			// Add usage
			log.Infoln("response usage reported: ", responseUsage)
			// Add usage
			addToUsage(tenantId, responseUsage)
		}(response, tenantId)
		w.Write(response)
	})
}
