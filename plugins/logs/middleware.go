package logs

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/console"
	"github.com/appbaseio/reactivesearch-api/model/difference"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/request"
	"github.com/appbaseio/reactivesearch-api/model/requestlogs"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/buger/jsonparser"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
}

// SearchResponseBody represents the response body returned by search
type SearchResponseBody struct {
	Took float64 `json:"took"`
}

// RSSettings represents the settings object in RS API response
type RSSettings struct {
	Took float64 `json:"took"`
}

// ResponseBodyRS represents the response body returned by reactivesearch route
type ResponseBodyRS struct {
	Settings RSSettings `json:"settings"`
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{

		classifyCategory,
		classify.Op(),
		classify.Indices(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.Logs

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

type Request struct {
	URI        string              `json:"uri"`
	Method     string              `json:"method"`
	HeadersMap map[string][]string `json:"header"`
	Headers    *string             `json:"headers_string"`
	Body       string              `json:"body"`
}

type Response struct {
	Code       int                 `json:"code"`
	Status     string              `json:"status"`
	HeadersMap map[string][]string `json:"header"`
	Headers    *string             `json:"headers_string"`
	Took       *float64            `json:"took,omitempty"`
	Body       string              `json:"body"`
}

type record struct {
	Indices                   []string                `json:"indices"`
	Category                  string                  `json:"category"`
	Request                   Request                 `json:"request"`
	Response                  Response                `json:"response"`
	RequestChanges            []difference.Difference `json:"requestChanges"`
	ResponseChanges           []difference.Difference `json:"responseChanges"`
	Timestamp                 time.Time               `json:"timestamp"`
	Console                   []string                `json:"console_logs"`
	IsUsingStringifiedHeaders bool                    `json:"is_using_stringified_headers"`
	TenantId                  string                  `json:"tenant_id"`
}

// Recorder records a log "record" for every request.
func Recorder() middleware.Middleware {
	return Instance().recorder
}

func (l *Logs) recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// skip logs from streams and blacklisted paths
		if r.Header.Get("X-Request-Category") == "streams" || isPathBlacklisted(r.URL.Path) {
			h(w, r)
			return
		}
		dumpRequest, err := httputil.DumpRequest(r, true)
		if err != nil {
			log.Errorln(logTag, ":", err.Error())
			return
		}

		// Init the console logs in the context
		consoleLogs := make([]string, 0)
		consoleLogsCtx := console.NewContext(r.Context(), &consoleLogs)
		r = r.WithContext(consoleLogsCtx)

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

		go l.recordResponse(respRecorder, r, dumpRequest)
	}
}

type Query struct {
	Type string `json:"type"`
}
type RSAPI struct {
	Query []Query `json:"query"`
}

type RequestChange struct {
	Before requestlogs.LogsResults
	After  requestlogs.LogsResults
}

func (l *Logs) recordResponse(w *httptest.ResponseRecorder, r *http.Request, reqBody []byte) {
	var headers = make(map[string][]string)

	for key, values := range r.Header {
		headers[key] = values
	}

	ctx := r.Context()

	reqCategory, err := category.FromContext(ctx)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}

	reqIndices, err := index.FromContext(ctx)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return
	}

	requestInfo, err := request.FromRequestIDContext(ctx)
	if err != nil {
		if *reqCategory == category.ReactiveSearch {
			log.Warnln(logTag, ":", err)
		}
	}

	var rec record
	tenantId, _ := util.GetAppbaseID()
	rec.TenantId = tenantId
	rec.Indices = reqIndices
	rec.Category = reqCategory.String()
	requestBody := strings.Split(string(reqBody), "\r\n\r\n")
	var parsedBody []byte
	if len(requestBody) > 1 {
		parsedBody = []byte(requestBody[1])
	}
	// apply suggestion category
	if *reqCategory == category.ReactiveSearch {
		var reqBody []byte
		if requestInfo != nil {
			reqBody = requestInfo.Body
			var query RSAPI
			err2 := json.Unmarshal(reqBody, &query)
			if err2 != nil {
				log.Errorln(logTag, ":", err2)
			} else {
				parsedBody = reqBody
				for _, query := range query.Query {
					if query.Type == "suggestion" {
						rec.Category = "suggestion"
					}
				}
			}
		}
	}
	rec.Timestamp = time.Now()

	// record response
	response := w.Result()
	rec.Response.Code = response.StatusCode
	rec.Response.Status = http.StatusText(response.StatusCode)
	rec.Response.Headers = GetStringifiedHeader(response.Header)

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorln(logTag, "can't read response body: ", err)
		return
	}
	if *reqCategory == category.Search {
		var resBody SearchResponseBody
		err := json.Unmarshal(responseBody, &resBody)
		if err != nil {
			log.Errorln(logTag, "error encountered while parsing the response: ", err)
		}
		// ignore error to record error logs
		if err == nil {
			rec.Response.Took = &resBody.Took
		}
	}
	requestBodyToStore := string(parsedBody[:util.Min(len(parsedBody), 1000000)])
	if *reqCategory == category.ReactiveSearch {
		rec.Request = Request{
			URI:     r.URL.Path,
			Headers: GetStringifiedHeader(headers),
			Body:    requestBodyToStore,
			Method:  r.Method,
		}
		// read success response from context
		tookValue, err := jsonparser.GetFloat(w.Body.Bytes(), "settings", "took")
		if err != nil {
			log.Warnln(logTag, "error encountered while reading took key from response body:", err)
		} else {
			// Set took value
			rec.Response.Took = &tookValue
		}
		// read error response from response recorder body
		rec.Response.Body = string(responseBody[:util.Min(len(responseBody), 1000000)])
	} else {
		// record request
		rec.Request = Request{
			URI:     r.URL.Path,
			Headers: GetStringifiedHeader(headers),
			Body:    requestBodyToStore,
			Method:  r.Method,
		}
		rec.Response.Body = string(responseBody[:util.Min(len(responseBody), 1000000)])
	}

	// Extract the request changes from context
	if requestInfo != nil {
		requestId := requestInfo.Id
		rl := requestlogs.Get(requestId)
		if rl != nil {
			go func() {
				rl.LogsDiffing.Wait()
				close(rl.Output)
			}()
			requestChangesByStage := make(map[string]RequestChange)
			responseChangesByStage := make(map[string]RequestChange)

			for result := range rl.Output {
				if result.LogType == "request" {
					requestChange := RequestChange{}
					rc, ok := requestChangesByStage[result.Stage]
					if ok {
						requestChange = rc
					}
					if result.LogTime == "before" {
						requestChange.Before = result
					} else if result.LogTime == "after" {
						requestChange.After = result
					}
					requestChangesByStage[result.Stage] = requestChange
				} else if result.LogType == "response" {
					responseChange := RequestChange{}
					rc, ok := requestChangesByStage[result.Stage]
					if ok {
						responseChange = rc
					}
					if result.LogTime == "before" {
						responseChange.Before = result
					} else if result.LogTime == "after" {
						responseChange.Before = result
					}
					responseChangesByStage[result.Stage] = responseChange
				}
			}
			// calculate diffing
			requestChanges := make([]difference.Difference, 0)
			for stage, changes := range requestChangesByStage {
				requestChanges = append(requestChanges, difference.Difference{
					Body:    util.CalculateBodyStringDiff(changes.Before.Data.Body, changes.After.Data.Body),
					Headers: util.CalculateHeaderDiff(changes.Before.Data.Headers, changes.After.Data.Headers),
					URI:     util.CalculateStringDiff(changes.Before.Data.URL, changes.After.Data.URL),
					Method:  util.CalculateMethodStringDiff(changes.Before.Data.Method, changes.After.Data.Method),
					Stage:   stage,
					Took:    &changes.After.TimeTaken,
				})
			}
			rec.RequestChanges = requestChanges
			responseChanges := make([]difference.Difference, 0)
			for stage, changes := range responseChangesByStage {
				responseChanges = append(responseChanges, difference.Difference{
					Body:    util.CalculateBodyStringDiff(changes.Before.Data.Body, changes.After.Data.Body),
					Headers: util.CalculateHeaderDiff(changes.Before.Data.Headers, changes.After.Data.Headers),
					Stage:   stage,
					Took:    &changes.After.TimeTaken,
				})
			}
			rec.ResponseChanges = responseChanges

			// Delete request logs
			requestlogs.Delete(requestId)
		}
	}

	// Extract the console logs
	consoleStr, err := console.FromContext(ctx)
	if err != nil {
		log.Warnln(logTag, "couldn't extract console logs, ", err)
	} else {
		rec.Console = *consoleStr
	}
	rec.IsUsingStringifiedHeaders = true
	marshalledLog, err := json.Marshal(rec)
	if err != nil {
		log.Warningln(logTag, "error encountered while marshalling record :", err)
		return
	}
	n, err := l.lumberjack.Write(marshalledLog)
	if err != nil {
		log.Errorln(logTag, "error encountered while writing logs :", err)
		return
	}
	// Add new line character so filebeat can sync it with ES
	l.lumberjack.Write([]byte("\n"))
	log.Println(logTag, "logged request successfully", n)
}

func GetStringifiedHeader(header http.Header) *string {
	marshalledHeaders, err := json.Marshal(header)
	if err != nil {
		log.Errorln(logTag, "error marshalling header", err)
		return nil
	}
	headerAsString := string(marshalledHeaders)
	return &headerAsString
}
