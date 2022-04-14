package telemetry

import (
	"encoding/json"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/model/tracktime"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/buger/jsonparser"
	badger "github.com/outcaste-io/badger/v3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Recorder records telemetry "record" for every request.
// Note: It must be the last middleware in all the plugins
func Recorder() middleware.Middleware {
	return Instance().recorder
}

// SearchResponseBody represents the response body returned by search
type SearchResponseBody struct {
	Took float64 `json:"took"`
}

// TelemetryRecord plugin records the API usage.
type TelemetryRecord struct {
	// timestamp in UNIX
	TimeStamp        int64  `json:"timestamp"`
	CPU              int64  `json:"cpu"`
	URL              string `json:"url"`
	Method           string `json:"m"`
	Category         string `json:"cat"`
	ServerStatusCode int64  `json:"ssc"`
	RunTime          string `json:"rt"`
	ServerMode       string `json:"mode"`
	Plan             string `json:"plan"`
	ServerVersion    string `json:"ver"`
	// Machine ID
	ServerID string `json:"sid"`
	// The following properties may present or not
	ClientIPv4     *string `json:"cip,omitempty"`
	ClientIPv6     *string `json:"cip6,omitempty"`
	FrontEndClient *string `json:"fe,omitempty"`
	// Memory allocated to service in MB(s)
	MEMORY *uint64 `json:"mem,omitempty"`
	// Response time taken by Elasticsearch for search requests in milliseconds
	SearchResponseTime *int64 `json:"srt,omitempty"`
	// Response time taken by RS API for search requests in milliseconds
	AppbaseResponseTime *int64 `json:"art,omitempty"`
	// Response size in bytes
	ServerResponseSize *int64 `json:"srs,omitempty"`
	// Disk Size in MB(s)
	AvailableDisk *uint64 `json:"disk,omitempty"`
	Acl           *string `json:"acl,omitempty"`
}

func WriteBackErrorWithTelemetry(req *http.Request, w http.ResponseWriter, err string, code int) {
	util.WriteBackError(w, err, code)
	respRecorder := httptest.NewRecorder()
	respRecorder.Code = code
	// call telemetry directly
	Instance().recorderError(respRecorder, req)
}

// records the telemetry for handlers
func (t *Telemetry) recorderError(w *httptest.ResponseRecorder, r *http.Request) {
	if util.IsTelemetryEnabled &&
		r.Header.Get(telemetryHeader) != "false" &&
		!util.Contains(blacklistRoutes, r.RequestURI) {
		go t.recordTelemetry(w, r)
	}
}

func (t *Telemetry) recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if util.IsTelemetryEnabled &&
			r.Header.Get(telemetryHeader) != "false" &&
			!util.Contains(blacklistRoutes, r.RequestURI) {
			// Serve using response recorder
			respRecorder := httptest.NewRecorder()
			h(respRecorder, r)
			// Copy the response to writer
			for k, v := range respRecorder.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(respRecorder.Code)
			w.Write(respRecorder.Body.Bytes())
			// Record the document

			go t.recordTelemetry(respRecorder, r)
		} else {
			h(w, r)
		}
	}
}

func (t *Telemetry) recordTelemetry(w *httptest.ResponseRecorder, r *http.Request) {
	ctx := r.Context()

	// ---- Start Category Calculation: Required ----
	reqCategory, err := category.FromContext(ctx)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}
	// ---- End Category Calculation ----

	// ---- Start Server Mode and Plan Calculation: Required ----
	serverMode := getServerMode()
	var plan string
	if util.GetTier() != nil {
		plan = util.GetTier().String()
	}
	if serverMode == defaultServerMode {
		plan = "opensource"
	}
	// ---- End Server Mode and Plan Calculation ----

	// ---- Start ACL Calculation: Optional ----
	reqAcl, err := acl.FromContext(ctx)
	if err != nil {
		log.Warnln(logTag, ":", err)
	}
	var aclString *string
	if reqAcl != nil {
		a := reqAcl.String()
		aclString = &a
	}
	// ---- End ACL Calculation ----

	// ---- Start Frontend Header Calculation: Optional ----
	var frontEndHeaderValue *string
	feHeader := r.Header.Get(frontEndHeader)
	if feHeader != "" {
		frontEndHeaderValue = &feHeader
	}
	// ---- End Frontend Header Calculation ----

	// ---- Start Response Size Calculation: Optional ----
	var responseSize *int64
	if w.Body != nil {
		s := int64(len(w.Body.Bytes()))
		responseSize = &s
	}
	// ---- End Response Size Calculation ----

	// ---- Start Allocated memory Calculation: Optional ----
	var memoryInMB *uint64
	if util.MemoryAllocated != 0 {
		m := util.MemoryAllocated / 1000000
		memoryInMB = &m
	}
	// ---- End Allocated memory Calculation ----

	// ---- Start Disk Space Calculation: Optional ----
	var availableDiskInMB *uint64
	var stats unix.Statfs_t
	wd, err3 := os.Getwd()
	// ignore error
	if err3 == nil {
		unix.Statfs(wd, &stats)
		// Available blocks * size per block = available space in bytes
		diskSizeInMB := (stats.Bavail * uint64(stats.Bsize)) / 1000000
		availableDiskInMB = &diskSizeInMB
	} else {
		log.Warnln(logTag, err3)
	}
	// ---- End Disk Space Calculation ----

	// ---- Start Search Response Time Calculation: Optional ----
	response := w.Result()

	var serarchResponseTime *int64

	if response.Body != nil {
		responseBody, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Errorln(logTag, "can't read response body: ", err)
			return
		}
		if *reqCategory == category.Search {
			var resBody SearchResponseBody
			err := json.Unmarshal(responseBody, &resBody)
			if err != nil {
				// ignore error
				log.Warnln(logTag, "error encountered while reading took key from search response: ", err)
			} else {
				took := int64(resBody.Took)
				serarchResponseTime = &took
			}
		} else if *reqCategory == category.ReactiveSearch {
			// read success response from context
			tookValue, err := jsonparser.GetFloat(w.Body.Bytes(), "settings", "took")
			if err != nil {
				log.Warnln(logTag, "error encountered while reading took key from reactivesearch response: ", err)
			} else {
				took := int64(tookValue)
				serarchResponseTime = &took
			}
		}
	}
	// ---- End Search Response Time Calculation ----

	// ---- Start Appbase Response Time Calculation: Optional ----
	var appbaseResponseTime *int64
	startTime, err := tracktime.FromTimeTrackerContext(r.Context())
	if err != nil {
		log.Warnln(logTag, "error encountered while reading start time from request context: ", err)
	} else {
		took := time.Since(*startTime).Milliseconds()
		appbaseResponseTime = &took
	}

	// ---- End Appbase Response Time Calculation ----

	// ---- Start IP Calculation: Optional* ----
	// Ipv4 or Ipv6 must be present

	ip := iplookup.FromRequest(r)

	var clientIPv4 *string
	ipv4 := getClientIP4(ip)
	if ipv4 != "" {
		clientIPv4 = &ipv4
	}

	var clientIPv6 *string
	ipv6 := getClientIP6(ip)
	if ipv6 != "" {
		clientIPv6 = &ipv6
	}

	// ---- End IP Calculation ----

	record := TelemetryRecord{
		TimeStamp:           time.Now().UnixNano() / int64(math.Pow(10, 6)), // unix timestamp in ms
		ClientIPv4:          clientIPv4,
		ClientIPv6:          clientIPv6,
		FrontEndClient:      frontEndHeaderValue,
		CPU:                 int64(runtime.NumCPU()),
		MEMORY:              memoryInMB,
		Plan:                plan,
		SearchResponseTime:  serarchResponseTime,
		AppbaseResponseTime: appbaseResponseTime,
		ServerResponseSize:  responseSize,
		ServerStatusCode:    int64(response.StatusCode),
		ServerID:            util.MachineID,
		AvailableDisk:       availableDiskInMB,
		URL:                 r.RequestURI,
		Method:              r.Method,
		Category:            reqCategory.String(),
		Acl:                 aclString,
		ServerVersion:       util.Version,
		ServerMode:          serverMode,
		RunTime:             util.RunTime,
	}

	var recordMap map[string]interface{}

	recordInBytes, errMarshal := json.Marshal(record)
	if errMarshal != nil {
		log.Errorln(logTag, ": ", errMarshal)
	}
	errUnmarshal := json.Unmarshal(recordInBytes, &recordMap)
	if errUnmarshal != nil {
		log.Errorln(logTag, ": ", errUnmarshal)
	}
	// ---- Add applied middlewares ----
	appliedMiddlewares := trackplugin.FrompluginTrackerContext(ctx)
	for _, v := range appliedMiddlewares {
		if len(v) >= 2 {
			recordMap["p_"+v[0:2]] = true
		}
	}
	var eventType string
	if getCustomer() != "" || util.Opensource == "true" {
		eventType = "telemetry_production"
	} else {
		eventType = "telemetry_staging"
	}
	recordMap["eventType"] = eventType
	recordInBytes, marshalErr := json.Marshal(recordMap)
	if marshalErr != nil {
		log.Errorln(logTag, " : ", marshalErr)
	}
	log.Println(recordMap)
	dbError := t.db.Update(func(txn *badger.Txn) error {
		key := []byte(strconv.FormatInt(time.Now().UnixNano(), 10))
		// record with a ttl value
		e := badger.NewEntry(key, recordInBytes).WithTTL(time.Minute*syncInterval + deltaInterval)
		err := txn.SetEntry(e)
		if err != nil {
			log.Errorln(logTag, "error while writing telemetry record to badger", recordInBytes)
		}
		return nil
	})
	if dbError != nil {
		log.Errorln(logTag, "error encountered while connecting badger", dbError)
	}
}
