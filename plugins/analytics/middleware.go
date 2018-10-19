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

const (
	XSearchQuery         = "X-Search-Query"
	XSearchId            = "X-Search-Id"
	XSearchFilters       = "X-Search-Filters"
	XSearchClick         = "X-Search-Click"
	XSearchClickPosition = "X-Search-Click-Position"
	XSearchConversion    = "X-Search-Conversion"
	XSearchCustomEvent   = "X-Search-Custom-Event"
)

// chain of middleware that are wrapped over analytics endpoints.
type chain struct {
	order.Fifo
}

// Wrap wraps a list of middleware over an http handler func.
func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

// list returns the list of middleware.
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
			Id     string                 `json:"id"`
		} `json:"hits"`
	} `json:"hits"`
}

type mSearchResponse struct {
	Responses []searchResponse `json:"responses"`
}

func (a *analytics) Recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ctxACL := ctx.Value(acl.CtxKey)
		if ctxACL == nil {
			log.Printf("%s: unable to fetch acl from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqACL, ok := ctxACL.(*acl.ACL)
		if !ok {
			log.Printf("%s: unable to cast context acl %v to *acl.ACL", logTag, reqACL)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		searchQuery := r.Header.Get(XSearchQuery)
		searchId := r.Header.Get(XSearchId)
		if *reqACL != acl.Search || (searchQuery == "" && searchId == "") {
			h(w, r)
			return
		}

		docId := searchId
		if docId == "" {
			docId = uuid.New().String()
		}

		// serve using response recorder
		respRecorder := httptest.NewRecorder()
		h(respRecorder, r)

		// copy the response to writer
		for k, v := range respRecorder.Header() {
			w.Header()[k] = v
		}
		w.Header().Set(XSearchId, docId)
		w.WriteHeader(respRecorder.Code)
		w.Write(respRecorder.Body.Bytes())

		go a.recordResponse(docId, searchId, respRecorder, r)
	}
}

// TODO: For urls ending with _search or _msearch? Stricter checks should make it hard to misuse
func (a *analytics) recordResponse(docId, searchId string, w *httptest.ResponseRecorder, r *http.Request) {
	// read the response from elasticsearch
	respBody, err := ioutil.ReadAll(w.Result().Body)
	if err != nil {
		log.Printf("%s: can't read response body, unable to record es response: %v", logTag, err)
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
			log.Printf("%s: can't unmarshal '_msearch' reponse, unable to record es response %s: %v",
				logTag, string(respBody), err)
			return
		}
		// TODO: why?
		if len(m.Responses) > 0 {
			esResponse = m.Responses[0]
		}
	} else {
		err := json.Unmarshal(respBody, &esResponse)
		if err != nil {
			log.Printf("%s: can't unmarshal '_search' reponse, unable to record es response %s: %v",
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
			log.Printf("%s: unable to marshal es response source %s: %v", logTag, source, err)
			continue
		}

		h := make(map[string]string)
		h["id"] = esResponse.Hits.Hits[i].Id
		h["type"] = esResponse.Hits.Hits[i].Type
		h["source"] = string(raw)
		hits = append(hits, h)
	}

	record := make(map[string]interface{})
	record["took"] = esResponse.Took
	if searchId == "" {
		ctxIndices := r.Context().Value(index.CtxKey)
		if ctxIndices == nil {
			log.Printf("%s: cannot fetch indices from request context, failed to record analytics",
				logTag)
			return
		}
		indices, ok := ctxIndices.([]string)
		if !ok {
			log.Printf("%s: unable to cast ctxIndices to []string, failed to record analytics",
				logTag)
			return
		}

		record["indices"] = indices
		record["search_query"] = r.Header.Get(XSearchQuery)
		record["hits_in_response"] = hits
		record["total_hits"] = esResponse.Hits.Total
		record["datestamp"] = time.Now().Format("2006/01/02 15:04:05")

		searchFilters := parse(r.Header.Get(XSearchFilters))
		if len(searchFilters) > 0 {
			record["search_filters"] = searchFilters
		}
	}

	ipAddr := iplookup.FromRequest(r)
	record["ip"] = ipAddr
	ipInfo := iplookup.Instance()

	coordinates, err := ipInfo.GetCoordinates(ipAddr)
	if err != nil {
		log.Printf("%s: error fetching location coordinates for ip=%s: %v", logTag, ipAddr, err)
	} else {
		record["location"] = coordinates
	}

	country, err := ipInfo.Get(iplookup.Country, ipAddr)
	if err != nil {
		log.Printf("%s: error fetching country for ip=%s: %v", logTag, ipAddr, err)
	} else {
		record["country"] = country
	}

	searchClick := r.Header.Get(XSearchClick)
	if searchClick != "" {
		if clicked, err := strconv.ParseBool(searchClick); err == nil {
			record["click"] = clicked
		} else {
			log.Printf("%s: invalid bool value '%v' passed for header %s: %v",
				logTag, searchClick, XSearchClick, err)
		}
	}

	searchClickPosition := r.Header.Get(XSearchClickPosition)
	if searchClickPosition != "" {
		if pos, err := strconv.Atoi(searchClickPosition); err == nil {
			record["click_position"] = pos
		} else {
			log.Printf("%s: invalid int value '%v' passed for header %s: %v",
				logTag, searchClickPosition, XSearchClickPosition, err)
		}
	}

	searchConversion := r.Header.Get(XSearchConversion)
	if searchConversion != "" {
		if conversion, err := strconv.ParseBool(searchConversion); err == nil {
			record["conversion"] = conversion
		} else {
			log.Printf("%s: invalid bool value '%v' passed for header %s: %v",
				logTag, searchConversion, XSearchConversion, err)
		}
	}

	customEvents := parse(r.Header.Get(XSearchCustomEvent))
	if len(customEvents) > 0 {
		record["custom_events"] = customEvents
	}

	logRaw(record) // TODO: remove
	a.es.indexRecord(docId, record)
}

func classifyACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestACL := acl.Analytics

		ctx := r.Context()
		ctx = context.WithValue(ctx, acl.CtxKey, &requestACL)
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

		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser == nil {
			log.Printf("%s: cannot fetch user object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqUser, ok := ctxUser.(*user.User)
		if !ok {
			log.Printf("%s: cannot cast ctxUser to *user.User", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ctxOp := ctx.Value(op.CtxKey)
		if ctxOp == nil {
			log.Printf("%s: cannot fetch op from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqOp, ok := ctxOp.(*op.Operation)
		if !ok {
			log.Printf("%s: cannot cast ctxOp to *op.Operation", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !reqUser.Can(*reqOp) {
			msg := fmt.Sprintf(`user with "user_id"="%s" does not have "%s" op access`,
				reqUser.UserId, reqOp.String())
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser == nil {
			log.Printf("%s: cannot fetch user object from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqUser, ok := ctxUser.(*user.User)
		if !ok {
			log.Printf("%s: cannot cast ctxUser to *user.User", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if !reqUser.HasACL(acl.Analytics) {
			msg := fmt.Sprintf(`user with "user_id"="%s" does not have "%s" acl`,
				reqUser.UserId, acl.Analytics.String())
			util.WriteBackMessage(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, r)
	}
}

func validateIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		ctxUser := ctx.Value(user.CtxKey)
		if ctxUser == nil {
			log.Printf("%s: unable to fetch permission from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		reqUser, ok := ctxUser.(*user.User)
		if !ok {
			log.Printf("%s: unable to cast context user to *user.User", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ctxIndices := ctx.Value(index.CtxKey)
		if ctxIndices == nil {
			log.Printf("%s: unable to fetch indices from request context", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		indices, ok := ctxIndices.([]string)
		if !ok {
			log.Printf("%s: unable to cast context indices to []string", logTag)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if len(indices) == 0 {
			// cluster level route
			ok, err := reqUser.CanAccessIndex("*")
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
			if !ok {
				util.WriteBackMessage(w, "User is unauthorized to access cluster level routes",
					http.StatusUnauthorized)
				return
			}
		} else {
			// index level route
			for _, indexName := range indices {
				ok, err := reqUser.CanAccessIndex(indexName)
				if err != nil {
					msg := fmt.Sprintf("invalid index pattern encountered %s", indexName)
					log.Printf("%s: invalid index pattern encountered %s: %v", logTag, indexName, err)
					util.WriteBackMessage(w, msg, http.StatusUnauthorized)
					return
				}

				if !ok {
					msg := fmt.Sprintf(`User is unauthorized to access index names "%s"`, indexName)
					util.WriteBackMessage(w, msg, http.StatusUnauthorized)
					return
				}
			}
		}

		h(w, r)
	}
}
