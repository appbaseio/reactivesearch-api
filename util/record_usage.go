package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
var tenantToUsageMap = make(map[string]int)

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
func FetchUsageForDay() error {
	urlToHit := ACCAPI + "sls/report_usage_multi_tenant"

	req, err := http.NewRequest(http.MethodGet, urlToHit, nil)
	if err != nil {
		// Handle the error
		return err
	}

	req.Header.Add("X-REACTIVESEARCH-TOKEN", os.Getenv("REACTIVESEARCH_AUTH_TOKEN"))

	// Find the timestamp of the current day at 00:00:00
	beginningOfDay := time.Now().Truncate(24 * time.Hour)
	startTimestamp := beginningOfDay.Unix()

	// Adding a day to the beginning of the day would be the end of the
	// day.
	//
	// Essentially adding 24 hours to the beginning of the day will be the
	// end of the day.
	endTimestamp := beginningOfDay.Add(1 * 24 * time.Hour).Unix()

	// Apply query params
	urlValues := make(url.Values)
	urlValues["start_timestamp"] = []string{fmt.Sprintf("%d", startTimestamp)}
	urlValues["end_timestamp"] = []string{fmt.Sprintf("%d", endTimestamp)}
	q := urlValues
	req.URL.RawQuery = q.Encode()

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("cache-control", "no-cache")
	res, err := HTTPClient().Do(req)
	if err != nil {
		// Handle error
		return err
	}

	// Read the body
	resBody, readErr := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	if readErr != nil {
		// Handle error
		return fmt.Errorf("error while reading body for fetching usage: %s", readErr.Error())
	}

	usageResponse := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(resBody, &usageResponse)

	if unmarshalErr != nil {
		// Handle error
		return fmt.Errorf("error while unmarshalling usage data: %s", unmarshalErr.Error())
	}

	domainToUsage := make(map[string]int)

	for clusterId, usageValue := range usageResponse {
		// `usageValue` will be an array where there should be preferably
		// one element but if there are more than one we will add up the
		// `usage` value for all the elements.
		//
		// There should be technically one element because the usage is
		// returned on a per-day basis and we will pass the timestamp for
		// a day.
		usageAsArr, asArr := usageValue.([]interface{})
		if !asArr {
			log.Warnln(": error while parsing response for usage for cluster ID: ", clusterId)
			continue
		}

		totalUsage := 0
		for _, usageEach := range usageAsArr {
			usageEachAsMap, asMapOk := usageEach.(map[string]interface{})
			if !asMapOk {
				continue
			}

			usageAsInterface, isPresent := usageEachAsMap["usage"]
			if !isPresent {
				continue
			}

			usageAsFloat, asFloatOk := usageAsInterface.(float64)
			if !asFloatOk {
				continue
			}

			totalUsage += int(usageAsFloat)
		}

		domainToUsage[clusterId] = totalUsage
	}

	tenantToUsageMap = domainToUsage
	return nil
}

// GetUsageForTenant will return the usage for the passed
// tenant value.
//
// If the entry doesn't exist, 0 will be returned for the
// tenant
func GetUsageForTenant(tenantID string) int {
	usage, exists := tenantToUsageMap[tenantID]
	if !exists {
		return 0
	}

	return usage
}

// IsDataUsageExceeded will indicate true if the data usage
// of the tenant has exceeded the allowed limit based on
// their plan
func IsDataUsageExceeded(domain string) bool {
	// Fetch the plan for the passed tenant
	instanceDetails := GetSLSInstanceByDomain(domain)
	usageForTenant := GetUsageForTenant(GetTenantForDomain(domain))

	// Add a check to make sure that the instance details being
	// fetched are not invalid, if invalid, return the data usage
	// exceeded as `false`
	if instanceDetails == nil || instanceDetails.Tier == nil {
		return false
	}

	return instanceDetails.Tier.LimitForPlan().DataUsage.IsLimitExceeded(usageForTenant)
}
