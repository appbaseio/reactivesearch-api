package analytics

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/plugins/analyticsrequest"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
	"github.com/buger/jsonparser"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// Custom headers
const (
	XSearchQuery                    = "X-Search-Query"
	XSearchID                       = "X-Search-Id"
	XTimestamp                      = "X-timestamp"
	XSearchFilters                  = "X-Search-Filters"
	XSearchClickObjectID            = "X-Search-Click-Object"
	XSearchClick                    = "X-Search-Click"
	XSearchClickPosition            = "X-Search-ClickPosition"
	XSearchSuggestionsClick         = "X-Search-Suggestions-Click"
	XSearchSuggestionsClickPosition = "X-Search-Suggestions-ClickPosition"
	XSearchConversion               = "X-Search-Conversion"
	XSearchCustomEvent              = "X-Search-CustomEvent"
	XSearchState                    = "X-Search-State"
	XUserID                         = "X-User-Id"
)

type chain struct {
	middleware.Fifo
}

// A list of plans for which this feature will be available
var validPlans = []util.Plan{
	util.ArcEnterprise,
	util.HostedArcEnterprise,
	util.ProductionFirst2019,
	util.ProductionSecond2019,
	util.ProductionThird2019,
	util.ProductionFourth2019,
	// 2021 plans
	util.ProductionFirst2021,
	util.ProductionSecond2021,
	util.ProductionThird2021,
	util.HostedArcEnterprise2021,
	util.ProductionFirst2023,
}

// Middleware with custom filters (plan based) restriction
func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, append(list(), validatePlan)...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classify.Op(),
		classify.Indices(),
		logs.Recorder(),
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
		requestCategory := category.Analytics

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func validatePlan(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		restrictedRoutes := []string{
			"filter-labels",
			"filter-values",
			"insights",
			"insight-status",
		}
		var isRestrictedRoute bool
		for _, route := range restrictedRoutes {
			if strings.Contains(req.RequestURI, route) {
				isRestrictedRoute = true
				break
			}
		}

		// Throw error if custom filters are present in the URL
		if (isRestrictedRoute || len(getCustomFilters(req.URL.Query())) != 0) && !util.ValidatePlans(validPlans, util.GetFeatureCustomEvents()) {
			msg := "Custom filters feature is not available for the free plan users, please upgrade to a paid plan or remove custom filter."
			if util.GetTier() != nil {
				msg = "Custom filters feature is not available for the " + util.GetTier().String() + " plan users, please upgrade to a higher plan or remove custom filter."
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusPaymentRequired)
		} else {
			h(w, req)
		}
	}
}

// Recorder parses and records the search requests made to elasticsearch along with some other
// user information in order to calculate and serve useful analytics.
func Recorder() middleware.Middleware {
	return Instance().recorder
}

// Initialize the analytics record struct in request context
func (a *Analytics) initContext(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var searchQuery string
		var storedQueries []string
		ctx := analyticsrequest.NewContext(r.Context(), analyticsrequest.Record{
			SearchQuery:   &searchQuery,
			StoredQueries: &storedQueries,
		})
		r = r.WithContext(ctx)
		h(w, r)
	}
}

func (a *Analytics) recorder(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Check if recording analytics is enabled or not.
		if !IsAnalyticsEnabled() {
			h(w, r)
			return
		}

		reqACL, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "an error occurred while recording search request", http.StatusInternalServerError)
			return
		}
		//decode headers and set it back if it relates to analytics
		for header := range r.Header {
			if strings.HasPrefix(header, "X-Search-") {
				parsedValue, err := url.QueryUnescape(r.Header.Get(header))
				if err != nil {
					log.Errorln(logTag, ": error while decoding header:", err)
					h(w, r)
					return
				}
				r.Header.Set(header, parsedValue)
			}
		}
		if !(*reqACL == category.Search || *reqACL == category.ReactiveSearch) {
			h(w, r)
		} else {
			searchID := r.Header.Get(XSearchID)
			if *reqACL == category.Search && (r.Header[XSearchQuery] == nil && searchID == "") {
				h(w, r)
			} else {
				var isUsingExistingSearchID bool
				if *reqACL == category.ReactiveSearch {
					rsRequestBody, err := querytranslate.FromContext(r.Context())
					if err != nil {
						log.Errorln(logTag, ":", err)
						telemetry.WriteBackErrorWithTelemetry(r, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
						return
					}

					// Only record when recordAnalytics is set to true
					if rsRequestBody.Settings == nil || rsRequestBody.Settings.RecordAnalytics == nil || !*rsRequestBody.Settings.RecordAnalytics {
						h(w, r)
					} else if util.IsRSAPIValidateRoute(r) {
						// ignore analytics for validate route
						h(w, r)

					} else
					// If `x-search-id` header is not set and request body doesn't have search type of queries
					// then don't record analytics
					if strings.TrimSpace(searchID) != "" || HasSearchTypeOfQuery(rsRequestBody) {
						// Track plugin
						ctx := trackplugin.TrackPlugin(ctx, "an")
						r = r.WithContext(ctx)
						// serve using response recorder
						respRecorder := httptest.NewRecorder()
						h.ServeHTTP(respRecorder, r)
						// copy the response to writer
						for k, v := range respRecorder.Header() {
							w.Header()[k] = v
						}
						analyticsRecord, err2 := analyticsrequest.FromContext(r.Context())
						if err2 != nil {
							log.Errorln(logTag, ":", err)
							telemetry.WriteBackErrorWithTelemetry(r, w, "error encountered while retrieving analytics record from context", http.StatusInternalServerError)
							return
						}
						var searchQuery string
						if analyticsRecord.SearchQuery != nil {
							searchQuery = *analyticsRecord.SearchQuery
						}

						var userID string
						if rsRequestBody.Settings != nil && rsRequestBody.Settings.UserID != nil {
							userID = *rsRequestBody.Settings.UserID
						}

						docID := searchID
						if docID == "" {
							docIDFromSession, isUsingExistingSearchIDSession := a.getQueryIDByQueryTerm(r, searchQuery, userID)
							if docIDFromSession != nil {
								docID = *docIDFromSession
								isUsingExistingSearchID = isUsingExistingSearchIDSession
							}
						}
						if docID != "" {
							w.Header().Set(XSearchID, docID)
						}

						responseToWrite := respRecorder.Body.Bytes()
						// write queryId to response settings
						responseBody, err := jsonparser.Set(respRecorder.Body.Bytes(), []byte(fmt.Sprintf("%q", docID)), "settings", "queryId")
						if err != nil {
							log.Warnln(logTag, "unable to set queryId key in settings", err)
							responseBody2, err2 := jsonparser.Set(respRecorder.Body.Bytes(), []byte(fmt.Sprintf(`{ "queryId": %q }`, docID)), "settings")
							if err2 != nil {
								log.Warnln(logTag, "unable to set settings key", err2)
							} else {
								responseToWrite = responseBody2
							}
						} else {
							responseToWrite = responseBody
						}

						w.WriteHeader(respRecorder.Code)
						w.Write(responseToWrite)
						// record the search response
						if docID != "" {
							go a.recordResponse(docID, searchID, isUsingExistingSearchID, true, rsRequestBody, r, responseToWrite)
						}
					} else {
						h(w, r)
					}
				} else {
					// Track plugin
					ctx := trackplugin.TrackPlugin(ctx, "an")
					r = r.WithContext(ctx)
					// serve using response recorder
					respRecorder := httptest.NewRecorder()
					h.ServeHTTP(respRecorder, r)

					// copy the response to writer
					for k, v := range respRecorder.Header() {
						w.Header()[k] = v
					}
					userID := r.Header.Get(XUserID)
					docID := searchID
					searchQuery := r.Header.Get(XSearchQuery)
					if docID == "" {
						docIDFromSession, isUsingExistingSearchIDSession := a.getQueryIDByQueryTerm(r, searchQuery, userID)
						if docIDFromSession != nil {
							docID = *docIDFromSession
							isUsingExistingSearchID = isUsingExistingSearchIDSession
						}
					}
					if docID != "" {
						w.Header().Set(XSearchID, docID)
					}
					w.WriteHeader(respRecorder.Code)
					responseToWrite := respRecorder.Body.Bytes()
					w.Write(responseToWrite)

					// record the search response
					if docID != "" {
						go a.recordResponse(docID, searchID, isUsingExistingSearchID, false, nil, r, responseToWrite)
					}
				}
			}
		}
	}
}

// Returns true if request body has at least one type search query
func HasSearchTypeOfQuery(rsRequestBody *querytranslate.RSQuery) bool {
	for _, query := range rsRequestBody.Query {
		if query.Type == querytranslate.Search {
			return true
		}
	}
	return false
}

// Records the analytics and returns query id and response body with query id
func (a *Analytics) RecordAnalytics(
	request rules.ScriptRequest,
	rsRequestBody *querytranslate.RSQuery,
	response rules.ScriptResponse,
	indices []string,
) (*string, rules.ScriptResponse) {
	// Only record when recordAnalytics is set to true
	if rsRequestBody.Settings == nil || rsRequestBody.Settings.RecordAnalytics == nil || !*rsRequestBody.Settings.RecordAnalytics {
		return nil, response
	} else {
		r := http.Request{}
		// set indices from envs to context
		ctx := index.NewContext(r.Context(), indices)
		r = *r.WithContext(ctx)
		r.Header = make(http.Header)
		// apply headers from request headers
		for k, v := range request.Headers {
			r.Header.Set(k, v)
		}
		searchID := r.Header.Get(XSearchID)
		// If `x-search-id` header is not set and request body doesn't have search type of queries
		// then don't record analytics
		if strings.TrimSpace(searchID) == "" && !HasSearchTypeOfQuery(rsRequestBody) {
			return nil, response
		}
		envs := querytranslate.ExtractEnvsFromRequest(*rsRequestBody)
		var searchQuery string
		if envs.Query != nil {
			searchQuery = *envs.Query
		}
		// add analytics record in context
		r = *r.WithContext(analyticsrequest.NewContext(r.Context(), analyticsrequest.Record{
			SearchQuery: &searchQuery,
		}))
		var userID string
		if rsRequestBody.Settings != nil && rsRequestBody.Settings.UserID != nil {
			userID = *rsRequestBody.Settings.UserID
		}
		var isUsingExistingSearchID bool

		docID := searchID
		if docID == "" {
			docIDFromSession, isUsingExistingSearchIDSession := a.getQueryIDByQueryTerm(&r, searchQuery, userID)
			if docIDFromSession != nil {
				docID = *docIDFromSession
				isUsingExistingSearchID = isUsingExistingSearchIDSession
			}
		}
		var responseToWrite = []byte(response.Body)
		// write queryId to response settings
		responseBody, err := jsonparser.Set([]byte(response.Body), []byte(fmt.Sprintf("%q", docID)), "settings", "queryId")
		if err != nil {
			log.Warnln(logTag, "unable to set queryId key in settings", err)
			responseBody2, err2 := jsonparser.Set([]byte(response.Body), []byte(fmt.Sprintf(`{ "queryId": %q }`, docID)), "settings")
			if err2 != nil {
				log.Warnln(logTag, "unable to set settings key", err2)
			} else {
				responseToWrite = responseBody2
			}
		} else {
			responseToWrite = responseBody
		}

		// record the search response
		if docID != "" {
			go a.recordResponse(docID, searchID, isUsingExistingSearchID, true, rsRequestBody, &r, responseBody)
		}
		responseHeaders := response.Headers
		if responseHeaders == nil {
			responseHeaders = make(map[string]string)
		}
		responseHeaders[XSearchID] = docID
		responseWithQueryID := rules.ScriptResponse{
			Body:    string(responseToWrite),
			Headers: responseHeaders,
			Code:    response.Code,
		}
		return &docID, responseWithQueryID
	}
}
func (a *Analytics) getQueryIDByQueryTerm(r *http.Request, searchQuery string, userID string) (*string, bool) {
	var docID string
	if searchQuery != "" {
		/**
		Use the same search id if the following conditions met:
		1. Incoming query is exactly same as the saved query having an active session.
		2. Incoming query has just one character(last) more/less to the saved query having an active session.
		*/
		ipAddr := iplookup.FromRequest(r)
		if userID != "" {
			// use userID if present instead of ip address
			ipAddr = userID
		}
		// request timestamp passed from front-end
		var requestTimestamp *int64
		timestamp := r.Header.Get(XTimestamp)
		if timestamp != "" {
			timestampInt, err := strconv.Atoi(timestamp)
			if err == nil {
				timestampInt64 := int64(timestampInt)
				requestTimestamp = &timestampInt64
			}
		}
		if requestTimestamp != nil {
			// check the saved value of past timestamp of user
			pastTimestamp := a.timestampSession.Get(ipAddr)
			if pastTimestamp != 0 {
				// if past timestamp present
				// then the current request timestamp must be
				// greater than the past recorded time
				// if not then don't record analytics for such requests
				if pastTimestamp > *requestTimestamp {
					return nil, false
				}
			}
			// update the timestamp for active session
			a.timestampSession.Put(ipAddr, *requestTimestamp)
		}

		searchQueryLast := searchQuery[:len(searchQuery)-1]
		var searchQuerySecondLast string
		if len(searchQuery) >= 2 {
			searchQuerySecondLast = searchQuery[:len(searchQuery)-2]
		}
		sessionSearchKey := generateSessionKey(ipAddr, searchQuery)
		sessionSearchKeyLast := generateSessionKey(ipAddr, searchQueryLast)
		sessionSearchKeySecondLast := generateSessionKey(ipAddr, searchQuerySecondLast)
		if a.session.Get(sessionSearchKey) != "" {
			// search session found, exact match
			docID = a.session.Get(sessionSearchKey)
			return &docID, true
		}

		var sessionIDWithEditDistance *string

		// session found with edit distance of one
		// for e.g incoming query => harry
		// active session exists for => harr
		if searchQueryLast != "" && a.session.Get(sessionSearchKeyLast) != "" {
			docID := a.session.Get(sessionSearchKeyLast)
			sessionIDWithEditDistance = &docID
		}
		// session found with edit distance of two
		// for e.g incoming query => harry
		// active session doesn't exist for => harr
		// but, active session exists for => har
		if searchQuerySecondLast != "" && a.session.Get(sessionSearchKeySecondLast) != "" {
			docID := a.session.Get(sessionSearchKeySecondLast)
			sessionIDWithEditDistance = &docID
		}

		if sessionIDWithEditDistance != nil {
			whitlisted := []string{sessionSearchKey}
			/**
			We need to add the record to the lastest incoming query with the existing search id.
			Additionally, we would have to add a record with edit distance of one because it doesn't exist
			For example, if `x` search id is present for `har` & incoming query is
			`harry` then we have to add a session with key as `harry` and search id is `x`.
			Additionally, there would be one more session with `harr` and search id is `x`.
			Note: By creating a new record logically we're extending the current session by 30s,
			hence the next incoming query(`harryp`) will use the existing session for `harr`.
			Older records will automatically get deleted after 30s of the creation.
			*/
			a.session.Put(sessionSearchKey, *sessionIDWithEditDistance)
			if searchQueryLast != "" {
				a.session.Put(sessionSearchKeyLast, *sessionIDWithEditDistance)
				whitlisted = append(whitlisted, sessionSearchKeyLast)
			}
			if searchQuerySecondLast != "" {
				a.session.Put(sessionSearchKeySecondLast, *sessionIDWithEditDistance)
				whitlisted = append(whitlisted, sessionSearchKeySecondLast)
			}
			// Delete the stale sessions
			a.session.DeleteStaleSessions(whitlisted, ipAddr)
			return sessionIDWithEditDistance, true
		}

		docID = uuid.New().String()
		// Delete the stale sessions
		a.session.DeleteStaleSessions([]string{}, ipAddr)
		// Record the session with the newly created ID
		a.session.Put(sessionSearchKey, docID)
		if searchQueryLast != "" {
			// Record the -1 char prefix to support patterns like `harry => harr`
			a.session.Put(sessionSearchKeyLast, docID)
		}
		if searchQuerySecondLast != "" {
			// Record the -2 char prefix to support patterns like `harry => har`
			a.session.Put(sessionSearchKeySecondLast, docID)
		}
		return &docID, false
	}
	// create docID from empty_query
	docID = uuid.New().String()
	return &docID, false
}

func (a *Analytics) recordResponse(docID, searchID string, isUsingExistingSearchID bool, isRSAPI bool, rsRequestBody *querytranslate.RSQuery, r *http.Request, responseBody []byte) {
	switch util.GetVersion() {
	case 6:
		a.recordResponseEs6(docID, searchID, isUsingExistingSearchID, isRSAPI, rsRequestBody, r, responseBody)
	default:
		a.recordResponseEs7(docID, searchID, isUsingExistingSearchID, isRSAPI, rsRequestBody, r, responseBody)
	}
}

type RecordUserSessionConfig struct {
	userID                  string
	isUsingExistingSearchID bool
	numberOfFilters         int
	isClicked               bool
	r                       *http.Request
	customEvents            []analyticsrequest.CustomEvent
	queryID                 string
}

// Records a user session for a particular search request
func (a *Analytics) recordUserSession(config RecordUserSessionConfig) {
	// Don't record for invalid plans
	if !util.ValidatePlans(validPlans, util.GetFeatureCustomEvents()) {
		return
	}
	var effectiveUserID = config.userID
	ip := extractIPFromRequest(config.r)
	if effectiveUserID == "" {
		effectiveUserID = ip
	}
	ctxIndices, err := index.FromContext(config.r.Context())
	if err != nil {
		log.Errorln(logTag, ": cannot fetch indices from request context, ", err)
		return
	}
	// We use userID as the session key to store the active user sessions
	var docID string
	sessionKey := effectiveUserID

	// custom events as map
	var customEvents map[string]interface{}
	for _, v := range config.customEvents {
		if customEvents == nil {
			customEvents = map[string]interface{}{}
		}
		customEvents[v.Key] = v.Value
	}

	if a.userSession.Get(sessionKey) != nil {
		activeSession := a.userSession.Get(sessionKey)
		docID = activeSession.ID
		// Update last interaction time to current time
		lastInteractionTime := time.Now().Unix()
		duration := lastInteractionTime - activeSession.StartTime
		/* Sets the bounce to `false`
		A bounce property must be set to `false` in the following conditions:
		1. Incoming search query has a different `query` value i.e different search id
		2. An addition of a filter
		3. A click on result or suggestion
		*/
		bounce := activeSession.Bounce
		if !config.isUsingExistingSearchID || config.isClicked || config.numberOfFilters > 0 {
			bounce = false
		}
		// Update lastInteractionTime and duration in es
		err := a.es.updateUserSession(context.Background(), docID, UserSession{
			StartTime:           &activeSession.StartTime,
			LastInteractionTime: &lastInteractionTime,
			Duration:            &duration,
			UserID:              &effectiveUserID,
			IP:                  &ip,
			Indices:             &ctxIndices,
			Bounce:              &bounce,
			TimeStamp:           activeSession.TimeStamp,
		}, customEvents, config.queryID)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return
		}
		// delete the current active session
		a.userSession.Delete(sessionKey)
		// add the new session with next 30 mins validity
		a.userSession.Put(sessionKey, ActiveUserSession{
			ID:        docID,
			StartTime: activeSession.StartTime,
			Bounce:    bounce,
			TimeStamp: activeSession.TimeStamp,
		})
	} else {
		docID = uuid.New().String()
		startTime := time.Now().Unix()
		duration := int64(0)
		LastInteractionTime := time.Now().Unix() // last interaction time
		bounce := true                           // A new session will have bounce as `true`
		timestamp := time.Now().Format(time.RFC3339)
		// Create session to ES
		err := a.es.updateUserSession(context.Background(), docID, UserSession{
			StartTime:           &startTime,
			LastInteractionTime: &LastInteractionTime,
			Duration:            &duration,
			UserID:              &effectiveUserID,
			IP:                  &ip,
			Indices:             &ctxIndices,
			Bounce:              &bounce,
			TimeStamp:           timestamp,
		}, customEvents, config.queryID)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return
		}
		// Add the session into the list of active sessions
		a.userSession.Put(sessionKey, ActiveUserSession{
			ID:        docID,
			StartTime: startTime,
			Bounce:    true,
			TimeStamp: timestamp,
		})
	}
}
