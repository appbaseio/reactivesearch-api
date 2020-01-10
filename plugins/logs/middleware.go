package logs

import (
	"bytes"
	"context"
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
	"github.com/appbaseio/arc/plugins/auth"
	"github.com/appbaseio/arc/util"
)

type chain struct {
	middleware.Fifo
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
	Body    string `json:"body"`
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
		log.Println("======================================MIDDLEWARE: LOG==================================")
		// skip logs from streams
		if r.Header.Get("X-Request-Category") == "streams" {
			h(w, r)
			return
		}

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

		request := Request{
			URI:     r.URL.Path,
			Headers: headers,
			Body:    string(reqBody),
			Method:  r.Method,
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

		// Record the document
		go l.recordResponse(&request, respRecorder, r)
	}
}

func (l *Logs) recordResponse(request *Request, w *httptest.ResponseRecorder, req *http.Request) {
	ctx := req.Context()

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

	// record request
	rec.Request = *request

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
	rec.Response.Body = string(responseBody)
	l.es.indexRecord(context.Background(), rec)
}
