package logs

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/middleware/classify"
	"github.com/appbaseio/arc/middleware/validate"
	"github.com/appbaseio/arc/model/category"
	"github.com/appbaseio/arc/model/index"
	"github.com/appbaseio/arc/model/request"
	"github.com/appbaseio/arc/model/requestid"
	"github.com/appbaseio/arc/model/response"
	"github.com/appbaseio/arc/plugins/auth"
	"github.com/appbaseio/arc/util"
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
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.User

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

type Request struct {
	URI     string              `json:"uri"`
	Method  string              `json:"method"`
	Headers map[string][]string `json:"header"`
	Body    string              `json:"body"`
}

type Response struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Headers map[string][]string
	Took    *float64 `json:"took,omitempty"`
	Body    string   `json:"body"`
}

type record struct {
	Indices   []string          `json:"indices"`
	Category  category.Category `json:"category"`
	Request   Request           `json:"request"`
	Response  Response          `json:"response"`
	Timestamp time.Time         `json:"timestamp"`
}

// Recorder records a log "record" for every request.
func Recorder() middleware.Middleware {
	return Instance().recorder
}

func (l *Logs) recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// skip logs from streams
		if r.Header.Get("X-Request-Category") == "streams" {
			h(w, r)
			return
		}
		ctx := r.Context()

		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return
		}
		requestID, err := requestid.FromContext(r.Context())
		if err != nil {
			log.Errorln(logTag, "request id not found in context :", err)
			return
		}
		// Serve using response recorder
		respRecorder := httptest.NewRecorder()
		h(respRecorder, r)

		// Copy the response to writer
		for k, v := range respRecorder.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(respRecorder.Code)
		w.Write(respRecorder.Body.Bytes())

		var rsResponseBody *map[string]interface{}
		if *reqCategory == category.ReactiveSearch {
			rsResponseBody = response.GetResponse(*requestID)
			if rsResponseBody == nil {
				log.Errorln(logTag, ":", "error reading response body")
				return
			}
		}
		// Record the document
		go l.recordResponse(respRecorder, r, *rsResponseBody)
	}
}

func (l *Logs) recordResponse(w *httptest.ResponseRecorder, r *http.Request, rsResponseBody map[string]interface{}) {
	// Read the request body
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorln(logTag, ": unable to read request body: ", err)
		util.WriteBackError(w, "Can't read request body", http.StatusInternalServerError)
		return
	}

	r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

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

	var rec record
	rec.Indices = reqIndices
	rec.Category = *reqCategory
	rec.Timestamp = time.Now()

	// record response
	response := w.Result()
	rec.Response.Code = response.StatusCode
	rec.Response.Status = http.StatusText(response.StatusCode)
	rec.Response.Headers = response.Header

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
	if *reqCategory == category.ReactiveSearch {
		// Read request body from context
		rsRequestBody, err := request.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, "error encountered while reading request body:", err)
			return
		}
		marshalled, err := json.Marshal(rsRequestBody)
		if err != nil {
			log.Errorln(logTag, "error encountered while marshalling request body:", err)
			return
		}
		rec.Request = Request{
			URI:     r.URL.Path,
			Headers: headers,
			Body:    string(marshalled[:util.Min(len(marshalled), 1000000)]),
			Method:  r.Method,
		}
		// ignore error to record error logs
		if err == nil {
			took, ok := rsResponseBody["settings"].(map[string]interface{})["took"].(float64)
			if !ok {
				log.Errorln(logTag, "error encountered while parsing response body:", err)
				return
			}
			rec.Response.Took = &took
		}
		marshalledRes, err := json.Marshal(rsResponseBody)
		if err != nil {
			log.Errorln(logTag, "error encountered while marshalling response body:", err)
			return
		}
		rec.Response.Body = string(marshalledRes[:util.Min(len(marshalledRes), 1000000)])
	} else {
		// record request
		rec.Request = Request{
			URI:     r.URL.Path,
			Headers: headers,
			Body:    string(reqBody[:util.Min(len(reqBody), 1000000)]),
			Method:  r.Method,
		}
		rec.Response.Body = string(responseBody[:util.Min(len(responseBody), 1000000)])
	}
	marshalledLog, err := json.Marshal(rec)
	if err != nil {
		log.Errorln(logTag, "error encountered while marshalling record :", err)
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
