package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/internal/iplookup"
	"github.com/appbaseio-confidential/arc/internal/types/acl"
	"github.com/appbaseio-confidential/arc/internal/types/index"
	"github.com/appbaseio-confidential/arc/internal/types/op"
	"github.com/appbaseio-confidential/arc/internal/types/user"
	"github.com/appbaseio-confidential/arc/internal/util"
	"github.com/appbaseio-confidential/arc/middleware/classifier"
	"github.com/appbaseio-confidential/arc/middleware/logger"
	"github.com/appbaseio-confidential/arc/middleware/path"
	"github.com/appbaseio-confidential/arc/plugins/auth"
	"github.com/google/uuid"
)

// Custom headers
const (
	XSearchQuery         = "X-Search-Query"
	XSearchID            = "X-Search-Id"
	XSearchFilters       = "X-Search-Filters"
	XSearchClick         = "X-Search-Click"
	XSearchClickPosition = "X-Search-Click-Position"
	XSearchConversion    = "X-Search-Conversion"
	XSearchCustomEvent   = "X-Search-Custom-Event"
)

type chain struct {
	order.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	basicAuth := auth.Instance().BasicAuth
	classifyOp := classifier.Instance().OpClassifier
	logRequests := logger.Instance().Log
	cleanPath := path.Clean

	return []middleware.Middleware{
		cleanPath,
		logRequests,
		classifyOp,
		classifyACL,
		extractIndices,
		basicAuth,
		validateOp,
		validateACL,
		validateIndices,
	}
}

type searchResponse struct {
	Took float64 `json:"took"`
	Hits struct {
		Total int `json:"total"`
		Hits  []struct {
			Source map[string]interface{} `json:"source"`
			Type   string                 `json:"type"`
			ID     string                 `json:"id"`
		} `json:"hits"`
	} `json:"hits"`
}

type mSearchResponse struct {
	Responses []searchResponse `json:"responses"`
}

// Recorder parses and records the search requests made to elasticsearch along with some other
// user information in order to calculate and serve useful analytics.
func (a *Analytics) Recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "An error occurred while recording search request", http.StatusInternalServerError)
			return
		}

		searchQuery := r.Header.Get(XSearchQuery)
		searchID := r.Header.Get(XSearchID)
		if *reqACL != acl.Search || (searchQuery == "" && searchID == "") {
			h(w, r)
			return
		}

		docID := searchID
		if docID == "" {
			docID = uuid.New().String()
		}

		// serve using response recorder
		respRecorder := httptest.NewRecorder()
		h(respRecorder, r)

		// copy the response to writer
		for k, v := range respRecorder.Header() {
			w.Header()[k] = v
		}
		w.Header().Set(XSearchID, docID)
		w.WriteHeader(respRecorder.Code)
		w.Write(respRecorder.Body.Bytes())

		// record the search response
		go a.recordResponse(docID, searchID, respRecorder, r)
	}
}

// TODO: For urls ending with _search or _msearch? Stricter checks should make it hard to misuse
func (a *Analytics) recordResponse(docID, searchID string, w *httptest.ResponseRecorder, r *http.Request) {
	// read the response from elasticsearch
	respBody, err := ioutil.ReadAll(w.Result().Body)
	if err != nil {
		log.Printf("%s: can't read response body, unable to record es response: %v\n", logTag, err)
		return
	}

	// replace es response fields
	respBody = bytes.Replace(respBody, []byte("_source"), []byte("source"), -1)
	respBody = bytes.Replace(respBody, []byte("_type"), []byte("type"), -1)
	respBody = bytes.Replace(respBody, []byte("_id"), []byte("id"), -1)

	var esResponse searchResponse
	if strings.Contains(r.RequestURI, "_msearch") {
		var m mSearchResponse
		err := json.Unmarshal(respBody, &m)
		if err != nil {
			log.Printf(`%s: can't unmarshal "_msearch" reponse, unable to record es response %s: %v`,
				logTag, string(respBody), err)
			return
		}
		// TODO: why record only the first _msearch response?
		if len(m.Responses) > 0 {
			esResponse = m.Responses[0]
		}
	} else {
		err := json.Unmarshal(respBody, &esResponse)
		if err != nil {
			log.Printf(`%s: can't unmarshal "_search" reponse, unable to record es response %s: %v`,
				logTag, string(respBody), err)
			return
		}
	}

	// record up to top 10 hits
	var hits []map[string]string
	for i := 0; i < 10 && i < len(esResponse.Hits.Hits); i++ {
		source := esResponse.Hits.Hits[i].Source
		raw, err := json.Marshal(source)
		if err != nil {
			log.Printf("%s: unable to marshal es response source %s: %v\n", logTag, source, err)
			continue
		}

		hit := make(map[string]string)
		hit["id"] = esResponse.Hits.Hits[i].ID
		hit["type"] = esResponse.Hits.Hits[i].Type
		hit["source"] = string(raw)
		hits = append(hits, hit)
	}

	record := make(map[string]interface{})
	record["took"] = esResponse.Took
	if searchID == "" {
		ctxIndices := r.Context().Value(index.CtxKey)
		if ctxIndices == nil {
			log.Printf("%s: cannot fetch indices from request context, failed to record es response\n", logTag)
			return
		}
		indices, ok := ctxIndices.([]string)
		if !ok {
			log.Printf("%s: unable to cast context indices to []string, failed to record es response\n", logTag)
			return
		}

		record["indices"] = indices
		record["search_query"] = r.Header.Get(XSearchQuery)
		record["hits_in_response"] = hits
		record["total_hits"] = esResponse.Hits.Total
		record["timestamp"] = time.Now().Format(time.RFC3339)

		searchFilters := parse(r.Header.Get(XSearchFilters))
		log.Printf("%v\n", searchFilters)
		if len(searchFilters) > 0 {
			record["search_filters"] = searchFilters
		}
	}

	ipAddr := iplookup.FromRequest(r)
	record["ip"] = ipAddr
	ipInfo := iplookup.Instance()

	coordinates, err := ipInfo.GetCoordinates(ipAddr)
	if err != nil {
		log.Printf("%s: error fetching location coordinates for ip=%s: %v\n", logTag, ipAddr, err)
	} else {
		record["location"] = coordinates
	}

	country, err := ipInfo.Get(iplookup.Country, ipAddr)
	if err != nil {
		log.Printf("%s: error fetching country for ip=%s: %v\n", logTag, ipAddr, err)
	} else {
		record["country"] = country
	}

	searchClick := r.Header.Get(XSearchClick)
	if searchClick != "" {
		if clicked, err := strconv.ParseBool(searchClick); err == nil {
			record["click"] = clicked
		} else {
			log.Printf("%s: invalid bool value '%v' passed for header %s: %v\n",
				logTag, searchClick, XSearchClick, err)
		}
	}

	searchClickPosition := r.Header.Get(XSearchClickPosition)
	if searchClickPosition != "" {
		if pos, err := strconv.Atoi(searchClickPosition); err == nil {
			record["click_position"] = pos
		} else {
			log.Printf("%s: invalid int value '%v' passed for header %s: %v\n",
				logTag, searchClickPosition, XSearchClickPosition, err)
		}
	}

	searchConversion := r.Header.Get(XSearchConversion)
	if searchConversion != "" {
		if conversion, err := strconv.ParseBool(searchConversion); err == nil {
			record["conversion"] = conversion
		} else {
			log.Printf("%s: invalid bool value '%v' passed for header %s: %v\n",
				logTag, searchConversion, XSearchConversion, err)
		}
	}

	customEvents := parse(r.Header.Get(XSearchCustomEvent))
	if len(customEvents) > 0 {
		record["custom_events"] = customEvents
	}

	logRaw(record) // TODO: remove
	a.es.indexRecord(docID, record)
}

func classifyACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestACL := acl.Analytics
		ctx := context.WithValue(r.Context(), acl.CtxKey, &requestACL)
		r = r.WithContext(ctx)
		h(w, r)
	}
}

func extractIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		indices, ok := util.IndicesFromRequest(r)
		if !ok {
			indices = []string{}
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, index.CtxKey, indices)
		r = r.WithContext(ctx)

		h(w, r)
	}
}

func validateOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request op"
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
			msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" op`, reqUser.Username, *reqOp)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		errMsg := "An error occurred while validating request acl"
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !reqUser.HasACL(acl.Analytics) {
			msg := fmt.Sprintf(`User with "username"="%s" does not have "%s" acl`, reqUser.Username, acl.Analytics)
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
