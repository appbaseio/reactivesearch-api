package logs

import (
	"bytes"
	"context"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/middleware/classify"
	"github.com/appbaseio-confidential/arc/middleware/validate"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/index"
	"github.com/appbaseio-confidential/arc/plugins/auth"
	"github.com/appbaseio-confidential/arc/util"
)

type chain struct {
	order.Fifo
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

type record struct {
	Indices  []string          `json:"indices"`
	Category category.Category `json:"category"`
	Request  struct {
		URI     string              `json:"uri"`
		Method  string              `json:"method"`
		Headers map[string][]string `json:"header"`
		Body    string              `json:"body"`
	} `json:"request"`
	Response struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Headers map[string][]string
		Body    string `json:"body"`
	} `json:"response"`
	Timestamp time.Time `json:"timestamp"`
}

// Recorder records a log "record" for every request.
func Recorder() middleware.Middleware {
	return Instance().recorder
}

func (l *Logs) recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("%s: unable to read request body: %v\n", logTag, err)
			util.WriteBackError(w, "Can't read request body", http.StatusInternalServerError)
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(reqBody))

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
		go l.recordResponse(reqBody, respRecorder, r)
	}
}

func (l *Logs) recordResponse(reqBody []byte, w *httptest.ResponseRecorder, req *http.Request) {
	ctx := req.Context()

	reqCategory, err := category.FromContext(ctx)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		return
	}

	reqIndices, err := index.FromContext(ctx)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		return
	}

	var rec record
	rec.Indices = reqIndices
	rec.Category = *reqCategory
	rec.Timestamp = time.Now()

	// record request
	rec.Request.URI = req.URL.Path
	rec.Request.Headers = req.Header
	rec.Request.Method = req.Method
	rec.Request.Body = string(reqBody)

	// record response
	response := w.Result()
	rec.Response.Code = response.StatusCode
	rec.Response.Status = http.StatusText(response.StatusCode)
	rec.Response.Headers = response.Header

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("%s: can't read response body: %v", logTag, err)
		return
	}
	rec.Response.Body = string(responseBody)
	l.es.indexRecord(context.Background(), rec)
}
