package util

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/appbaseio/reactivesearch-api/model/domain"
	log "github.com/sirupsen/logrus"
)

type Usage struct {
	Usage map[string]int
	mu    sync.Mutex
}

// Domain to usage map
var totalUsage = Usage{Usage: make(map[string]int), mu: sync.Mutex{}}

// Keep record of usage on a per domain basis
var domainToUsageMap = make(map[string]int)

func addToUsage(domain string, usage int) {
	totalUsage.mu.Lock()
	defer totalUsage.mu.Unlock()
	if _, ok := totalUsage.Usage[domain]; ok {
		totalUsage.Usage[domain] += usage
	} else {
		totalUsage.Usage[domain] = usage
	}
}

// Method to clear reported usage
func ClearUsage(domain string) {
	totalUsage.mu.Lock()
	defer totalUsage.mu.Unlock()
	delete(totalUsage.Usage, domain)
}

// Returns the total usage by domain
func GetDataUsageByDomain(domain string) int {
	totalUsage.mu.Lock()
	defer totalUsage.mu.Unlock()
	if _, ok := totalUsage.Usage[domain]; ok {
		return totalUsage.Usage[domain]
	}
	return 0
}

// Returns the total usage
func GetDataUsage() map[string]int {
	return totalUsage.Usage
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

// FetchUsageForDay will fetch the usage for the day from AccAPI.
//
// The usage will be fetched only for the current day by passing
// timestamps.
func FetchUsageForDay() {
	urlToHit := ACCAPI + "/sls/report_usage_multi_tenant"

	req, err := http.NewRequest(http.MethodGet, urlToHit, nil)
	if err != nil {
		// TODO: Handle the error
	}

	req.Header.Add("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))

	// Find the timestamp of the current day at 00:00:00
	startTimestamp := time.Now()

	// Apply query params
	urlValues := make(url.Values)
	urlValues["start_timestamp"] = []string{}
	urlValues["end_timestamp"] = []string{}
	q := urlValues
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := HTTPClient().Do(req)
	if err != nil {
		// TODO: Handle error
	}

}
