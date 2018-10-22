package logs

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/util"
)

type record struct {
	Indices []string `json:"indices"`
	ACL     acl.ACL  `json:"acl"`
	Request struct {
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
	}
	Timestamp time.Time `json:"timestamp"`
}

type esResponse struct {
	Hits struct {
		Hits []struct {
			Source map[string]interface{} `json:"_source"`
			ID     string                 `json:"_id"`
			Type   string                 `json:"_type"`
			Score  float64                `json:"_score"`
			Index  string                 `json:"_index"`
		} `json:"hits"`
		Total    int     `json:"total"`
		MaxScore float64 `json:"max_score"`
	} `json:"hits"`
	TimedOut bool  `json:"timed_out"`
	Took     int64 `json:"took"`
}

// Recorder records a log "record" for every request.
func (l *Logs) Recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read the request body
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("%s: unable to read request body: %v\n", logTag, err)
			util.WriteBackError(w, "Can't read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()
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

func (l *Logs) recordResponse(reqBody []byte, w *httptest.ResponseRecorder, r *http.Request) {
	ctx := r.Context()

	reqACL, err := acl.FromContext(ctx)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		return
	}

	reqIndices, err := util.IndicesFromContext(ctx)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		return
	}

	var record record
	record.Indices = reqIndices
	record.ACL = *reqACL
	record.Timestamp = time.Now()

	// record request
	record.Request.URI = r.RequestURI
	record.Request.Headers = r.Header
	record.Request.Method = r.Method
	record.Request.Body = string(reqBody)

	// record response
	response := w.Result()
	record.Response.Code = response.StatusCode
	record.Response.Status = http.StatusText(response.StatusCode)
	record.Response.Headers = response.Header

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("%s: can't read response body: %v", logTag, err)
		return
	}
	// we unmarshal the response inorder to trim down the number of hits to 10
	if *reqACL == acl.Search || *reqACL == acl.Streams {
		var response esResponse
		err := json.Unmarshal(reqBody, &response)
		if err != nil {
			log.Printf("%s: error unmarshalling es response, unable to record logs: %v", logTag, err)
			return
		}
		if len(response.Hits.Hits) > 10 {
			response.Hits.Hits = response.Hits.Hits[0:10]
		}
		responseBody, err = json.Marshal(response)
		if err != nil {
			log.Printf("%s: error marshalling es response, unable to record logs: %v", logTag, err)
			return
		}
	}
	record.Response.Body = string(responseBody)

	log.Printf("%s: %v", logTag, string(responseBody)) // TODO: remove
	l.es.indexRecord(record)
}
