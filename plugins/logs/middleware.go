package logs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/internal/types/category"
	"github.com/appbaseio-confidential/arc/internal/types/index"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/appbaseio-confidential/arc/middleware/classifier"
	"github.com/appbaseio-confidential/arc/middleware/logger"
	"github.com/appbaseio-confidential/arc/middleware/path"
	"github.com/appbaseio-confidential/arc/plugins/auth"
)

type chain struct {
	order.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	cleanPath := path.Clean
	logRequests := logger.Instance().Log
	classifyOp := classifier.Instance().OpClassifier
	basicAuth := auth.Instance().BasicAuth

	return []middleware.Middleware{
		cleanPath,
		logRequests,
		classifyCategory,
		classifyOp,
		identifyIndices,
		basicAuth,
		validateIndices,
		validateOp,
		validateCategory,
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCategory := category.User
		ctx := context.WithValue(r.Context(), category.CtxKey, &requestCategory)
		r = r.WithContext(ctx)
		h(w, r)
	}
}

func identifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		indices := util.IndicesFromRequest(r)

		fmt.Println(indices)

		ctx := r.Context()
		ctx = context.WithValue(ctx, index.CtxKey, indices)
		r = r.WithContext(ctx)

		h(w, r)
	}
}


func validateCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "error occurred while validating request category"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !reqUser.HasCategory(category.User) {
			msg := fmt.Sprintf(`user with "username"="%s" does not have "%s" category`,
				reqUser.Username, category.Analytics)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "an error occurred while validating request op"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqOp, err := op.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !reqUser.CanDo(*reqOp) {
			msg := fmt.Sprintf(`user with "username"="%s" does not have "%s" op`, reqUser.Username, *reqOp)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request indices"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		ctxIndices := ctx.Value(index.CtxKey)
		if ctxIndices == nil {
			log.Printf("%s: unable to fetch indices from request context\n", logTag)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}
		indices, ok := ctxIndices.([]string)
		if !ok {
			log.Printf("%s: unable to cast context indices to []string\n", logTag)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if len(indices) == 0 {
			// cluster level route
			ok, err := reqUser.CanAccessIndex("*")
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, `Invalid index pattern "*"`, http.StatusUnauthorized)
				return
			}
			if !ok {
				util.WriteBackError(w, "User is unauthorized to access cluster level routes", http.StatusUnauthorized)
				return
			}
		} else {
			// index level route
			for _, indexName := range indices {
				ok, err := reqUser.CanAccessIndex(indexName)
				if err != nil {
					msg := fmt.Sprintf(`Invalid index pattern encountered "%s"`, indexName)
					log.Printf("%s: invalid index pattern encountered %s: %v\n", logTag, indexName, err)
					util.WriteBackError(w, msg, http.StatusUnauthorized)
					return
				}

				if !ok {
					msg := fmt.Sprintf(`User is unauthorized to access index names "%s"`, indexName)
					util.WriteBackError(w, msg, http.StatusUnauthorized)
					return
				}
			}
		}

		h(w, r)
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

type esResponse struct {
	Hits struct {
		Hits []struct {
			Source map[string]interface{} `json:"_source,omitempty"`
			ID     string                 `json:"_id,omitempty"`
			Type   string                 `json:"_type,omitempty"`
			Score  float64                `json:"_score,omitempty"`
			Index  string                 `json:"_index,omitempty"`
		} `json:"hits"`
		Total    int     `json:"total"`
		MaxScore interface{} `json:"max_score"`
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

	reqCategory, err := category.FromContext(ctx)
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
	record.Category = *reqCategory
	record.Timestamp = time.Now()

	// record request
	record.Request.URI = r.URL.Path
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
	if *reqCategory == category.Search || *reqCategory == category.Streams {
		var response esResponse
		err := json.Unmarshal(responseBody, &response)
		if err != nil {
			log.Printf("%s: error unmarshaling es response, unable to record logs: %v", logTag, err)
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
