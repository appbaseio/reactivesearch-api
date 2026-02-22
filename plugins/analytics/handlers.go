package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/appbaseio/reactivesearch-api/plugins/analyticsrequest"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/rules"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/gorilla/mux"
)

func (a *Analytics) getOverview() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching analytics overview"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())

		raw, err := a.es.analyticsOverview(req.Context(), queryParams, clickAnalytics, filters, indices...)
		if err != nil {
			msg := "error occurred while aggregating analytics overview results"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getQueryOverview() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())
		// Return top 5 results and clicks if size is not defined
		defaultSize := 5
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			if value > 1000 {
				value = defaultSize
				log.Println(logTag, `: incoming "size" limit exceeded (> 1000), setting to the default value of 5 instead`)
			}
			size = value
		}
		queryParams.Size = size
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching query overview"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		q := req.URL.Query().Get("query")
		filters := getCustomFilters(req.URL.Query())

		raw, err := a.es.queryOverview(req.Context(), queryParams, q, filters, indices...)
		if err != nil {
			msg := "error occurred while aggregating query overview results"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getStoredQueriesUsage() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size
		filters := getCustomFilters(req.URL.Query())

		raw, err := a.es.storedQueriesUsage(req.Context(), queryParams, filters)
		if err != nil {
			msg := "error occurred while aggregating stored queries usage"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getRulesUsage() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())

		defaultSize := 1000
		var size = defaultSize
		respSize := req.URL.Query().Get("size")
		if respSize != "" {
			value, err := strconv.Atoi(respSize)
			if err != nil {
				value = defaultSize
				log.Errorln(logTag, `: invalid "size" value provided, defaulting to 5:`, err)
			}
			size = value
		}
		queryParams.Size = size
		filters := getCustomFilters(req.URL.Query())

		raw, err := a.es.queryRulesUsage(req.Context(), queryParams, filters)
		if err != nil {
			msg := "error occurred while aggregating query rules usage"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

// setPreferences will support setting analytics preferences
func (a *Analytics) setPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Read the request body
		defer req.Body.Close()
		var analyticsPreferences AnalyticsPreferences
		err := json.NewDecoder(req.Body).Decode(&analyticsPreferences)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, fmt.Sprintf("Can't parse request body: %v", err), http.StatusBadRequest)
			return
		}

		// Check if it is a local request and if so then only update
		// the cache and return.
		// To decide whether to just update the local state
		isLocal := req.URL.Query().Get("local")
		if isLocal == "true" {
			// Update local variable
			SetPreferencesInCache(analyticsPreferences)
			util.WriteBackMessage(w, "Analytics preferences saved successfully", http.StatusOK)
			return
		}

		// Validate the body.
		//
		// NOTE: As of now, only `enable` field is present so no need
		// to validate anything. When the body gets more fields,
		// validation should be added.
		if analyticsPreferences.Enable == nil {
			defaultEnable := true
			analyticsPreferences.Enable = &defaultEnable
		}

		// Save in ES
		saveErr := a.es.savePreferences(req.Context(), analyticsPreferences)
		if saveErr != nil {
			errMsg := fmt.Sprint("Error while saving the preferences in ES: ", saveErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		// Only update local state when proxy API has not been called
		// If proxy API would get called then it would automatically update the
		// state for all machines
		// Updating the local state again can cause inconsistency issues
		if util.ShouldProxyToACCAPI() {
			// Invoke ACCAPI

			// Convert the body into map.
			bodyInBytes, marshalErr := json.Marshal(analyticsPreferences)
			if marshalErr != nil {
				errMsg := fmt.Sprint("Error while marshalling analytics preferences: ", marshalErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			bodyAsMap := make(map[string]interface{})
			unmarshalErr := json.Unmarshal(bodyInBytes, &bodyAsMap)
			if unmarshalErr != nil {
				errMsg := fmt.Sprint("Error while unmarshalling analytics preferences: ", unmarshalErr.Error())
				log.Warnln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			res, err := util.ProxyACCAPI(util.ProxyConfig{
				Method: http.MethodPost,
				URL:    "/_analytics/preferences",
				Body:   bodyAsMap,
			})
			if err != nil {
				log.Errorln(logTag, ": Error creating proxy AccAPI body: ", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Failed to update all nodes, return error response
			if res != nil {
				log.Errorln(logTag, ":", "error encountered updating analytics preferences")
				bodyBytes, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				util.WriteBackRaw(w, bodyBytes, res.StatusCode)
				return
			}
		} else {
			// Update the cache
			SetPreferencesInCache(analyticsPreferences)
		}

		util.WriteBackMessage(w, "Preferences updated successfully!", http.StatusOK)
	}
}

// getPreferences will return the preferences for analytics
func (a *Analytics) getPreferences() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		preferences := GetPreferencesFromCache()

		preferencesAsBytes, marshalErr := json.Marshal(preferences)
		if marshalErr != nil {
			errMsg := fmt.Sprint("Error while marshalling preferences into bytes: ", marshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, preferencesAsBytes, http.StatusOK)
	}
}

func (a *Analytics) recordSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var body SearchRecord
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "Can't parse request body", http.StatusBadRequest)
			return
		}
		if body.Query == nil && body.QueryID == "" {
			telemetry.WriteBackErrorWithTelemetry(r, w, "Field 'query' or 'query_id' must be present, set query as empty string to register as an empty query search.", http.StatusBadRequest)
			return
		}
		// Set the priority to custom events
		customEvents := body.CustomEvents
		if customEvents == nil {
			customEvents = body.EventData
		}
		// validate custom events
		customEventValidatationErr := validateCustomEvents(customEvents)
		if customEventValidatationErr != nil {
			telemetry.WriteBackErrorWithTelemetry(r, w, customEventValidatationErr.Error(), http.StatusBadRequest)
			return
		}
		var queryInLowerCase *string
		if body.Query != nil {
			lowerCaseQuery := strings.ToLower(*body.Query)
			queryInLowerCase = &lowerCaseQuery
		}
		// build analytics record
		record := analyticsrequest.Record{
			SearchQuery:  queryInLowerCase,
			CustomEvents: extractCustomEvents(customEvents),
		}
		if body.Filters != nil {
			var filters []querytranslate.TermFilter
			for key, filter := range *body.Filters {
				// Use lower case for filter value
				filters = append(filters, querytranslate.TermFilter{
					Key:   key,
					Value: strings.ToLower(filter),
				})
			}
			record.SearchFilters = &filters
		}

		if body.Impressions != nil {
			var impressions []analyticsrequest.HIT
			for _, impression := range *body.Impressions {
				// validate impression id
				if impression.ID == "" {
					telemetry.WriteBackErrorWithTelemetry(r, w, "Field 'id' must be present in impression object.", http.StatusBadRequest)
					return
				}
				// validate impression index
				if impression.Index == "" {
					telemetry.WriteBackErrorWithTelemetry(r, w, "Field 'index' must be present in impression object.", http.StatusBadRequest)
					return
				}
				impressions = append(impressions, analyticsrequest.HIT{
					ID:    impression.ID,
					Index: impression.Index,
				})
			}
			record.HitsInResponse = &impressions
		}

		// We need to record total hits for query suggestions
		record.TotalHits = body.TotalHits
		vars := mux.Vars(r)
		record.Index = vars["index"]
		record.Indices = []string{vars["index"]}

		queryID := body.QueryID
		var isFromActiveSession bool
		if queryID == "" {
			queryIDSession, isFromActiveSession2 := a.getQueryIDByQueryTerm(r, *queryInLowerCase, body.UserID)
			if queryIDSession != nil {
				queryID = *queryIDSession
				isFromActiveSession = isFromActiveSession2
			} else {
				// return error response
				msg := "can not record analytics because a latest request has already been processed in the same session"
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(r, w, msg, http.StatusBadRequest)
				return
			}
		}

		if !isFromActiveSession && body.QueryID == "" {
			// populate meta properties for new records
			var err3 error
			record, err3 = addMetaDataToNewRecord(body.UserID, &record, r)
			if err3 != nil {
				log.Errorln(logTag, ":", err3)
				telemetry.WriteBackErrorWithTelemetry(r, w, err3.Error(), http.StatusInternalServerError)
				return
			}
		}
		// Update es record
		err2 := a.es.updateRecord(context.Background(), queryID, record)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			telemetry.WriteBackErrorWithTelemetry(r, w, err2.message.Error(), err2.code)
			return
		}
		var numberOfFilters int
		if record.SearchFilters != nil {
			numberOfFilters = len(*record.SearchFilters)
		}
		// Record user session
		go a.recordUserSession(RecordUserSessionConfig{
			userID:                  record.UserID,
			isUsingExistingSearchID: isFromActiveSession,
			numberOfFilters:         numberOfFilters,
			isClicked:               false,
			r:                       r,
			customEvents:            record.CustomEvents,
			queryID:                 queryID,
		})

		message := map[string]interface{}{
			"query_id": queryID,
		}
		raw, err := json.Marshal(message)
		if err != nil {
			msg := "error occurred while recording search analytics"
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) recordClick() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		var body ClickRecord
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "Can't parse request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		res := a.RecordClick(r, body)
		if res.Code != http.StatusOK {
			log.Errorln(logTag, ":", res.Body)
			telemetry.WriteBackErrorWithTelemetry(r, w, res.Body, res.Code)
			return
		} else {
			util.WriteBackMessage(w, res.Body, http.StatusOK)
		}
	}
}

func (a *Analytics) RecordClick(r *http.Request, body ClickRecord) rules.ScriptResponse {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json; charset=utf-8"
	headers["X-Content-Type-Options"] = "nosniff"
	// Query or query id must be present
	if body.Query == nil && body.QueryID == "" {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field 'query' or 'query_id' must be present, set query as empty string to register as an empty query click.",
			Headers: headers,
		}
	}
	// Check if click_on value is present
	if body.ClickOn == nil || (body.ClickOn != nil && len(*body.ClickOn) < 1) {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field 'click_on' can't be empty.",
			Headers: headers,
		}
	}
	// Set the priority to custom events
	customEvents := body.CustomEvents
	if customEvents == nil {
		customEvents = body.EventData
	}
	// validate custom events
	customEventValidationErr := validateCustomEvents(customEvents)
	if customEventValidationErr != nil {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    customEventValidationErr.Error(),
			Headers: headers,
		}
	}
	var queryInLowerCase *string
	if body.Query != nil {
		lowerCaseQuery := strings.ToLower(*body.Query)
		queryInLowerCase = &lowerCaseQuery
	}
	// Use query_id if present
	queryID := body.QueryID
	var isFromActiveSession bool
	if queryID == "" {
		queryIDSession, isFromActiveSession2 := a.getQueryIDByQueryTerm(r, *queryInLowerCase, body.UserID)
		if queryIDSession != nil {
			queryID = *queryIDSession
			isFromActiveSession = isFromActiveSession2
		} else {
			// return error response
			msg := "can not record analytics because a latest request has already been processed in the same session"
			return rules.ScriptResponse{
				Code:    http.StatusInternalServerError,
				Body:    msg,
				Headers: headers,
			}
		}
	}
	// build analytics record
	record := analyticsrequest.Record{
		SearchQuery:  queryInLowerCase,
		CustomEvents: extractCustomEvents(customEvents),
		Meta:         body.Meta,
	}
	vars := mux.Vars(r)
	record.Index = vars["index"]
	record.Indices = []string{vars["index"]}
	if !isFromActiveSession && body.QueryID == "" {
		// populate meta properties for new records
		var err2 error
		record, err2 = addMetaDataToNewRecord(body.UserID, &record, r)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			return rules.ScriptResponse{
				Code:    http.StatusBadRequest,
				Body:    err2.Error(),
				Headers: headers,
			}
		}
	}
	objectIds := extractObjectIds(*body.ClickOn)
	objectPositions := extractClickPositions(*body.ClickOn)
	if body.ClickType == Suggestion {
		record.SuggestionsClickObjectIds = objectIds
		record.SuggestionsClickPositionIds = objectPositions
	} else {
		record.ResultClickObjectIds = objectIds
		record.ResultClickPositionIds = objectPositions
	}
	// Update es record
	err2 := a.es.updateRecord(context.Background(), queryID, record)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return rules.ScriptResponse{
			Code:    err2.code,
			Body:    err2.message.Error(),
			Headers: headers,
		}
	}
	var isClicked bool
	if len(record.ResultClickObjectIds) != 0 || len(record.SuggestionsClickObjectIds) != 0 {
		isClicked = true
	}
	// Record user session
	go a.recordUserSession(RecordUserSessionConfig{
		userID:                  record.UserID,
		isUsingExistingSearchID: isFromActiveSession,
		numberOfFilters:         0,
		isClicked:               isClicked,
		r:                       r,
		customEvents:            record.CustomEvents,
		queryID:                 queryID,
	})
	return rules.ScriptResponse{
		Code:    http.StatusOK,
		Body:    "Click analytics recorded",
		Headers: headers,
	}
}

func (a *Analytics) recordConversion() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var body ConversionRecord
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(r, w, "Can't parse request body", http.StatusBadRequest)
			return
		}
		res := a.RecordConversion(r, body)
		if res.Code != http.StatusOK {
			log.Errorln(logTag, ":", res.Body)
			telemetry.WriteBackErrorWithTelemetry(r, w, res.Body, res.Code)
			return
		} else {
			util.WriteBackMessage(w, res.Body, http.StatusOK)
		}
	}
}

func (a *Analytics) RecordConversion(r *http.Request, body ConversionRecord) rules.ScriptResponse {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json; charset=utf-8"
	headers["X-Content-Type-Options"] = "nosniff"

	// Query or query id must be present
	if body.QueryID == "" {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field 'query_id' must be present",
			Headers: headers,
		}
	}
	// Check if click_on value is present
	if body.ConversionOn == nil || (body.ConversionOn != nil && len(*body.ConversionOn) < 1) {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field 'conversion_on' property can't be empty.",
			Headers: headers,
		}
	}

	// Use query_id if present
	queryID := body.QueryID
	// build analytics record
	record := analyticsrequest.Record{
		ConversionObjectIds: *body.ConversionOn,
		Meta:                body.Meta,
	}
	vars := mux.Vars(r)
	record.Index = vars["index"]

	// Update es record
	err2 := a.es.updateConversion(context.Background(), queryID, record)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return rules.ScriptResponse{
			Code:    err2.code,
			Body:    err2.message.Error(),
			Headers: headers,
		}
	}
	return rules.ScriptResponse{
		Code:    http.StatusOK,
		Body:    "Conversion recorded",
		Headers: headers,
	}
}

func (a *Analytics) getAdvanced() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching advanced analytics"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())

		raw, err := a.es.advancedAnalytics(req.Context(), queryParams, clickAnalytics, filters, indices...)
		if err != nil {
			msg := "error occurred while aggregating advanced analytics results"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		queryParams := rangeQueryParams(req.URL.Query())
		filters := getCustomFilters(req.URL.Query())

		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching popular searches"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.popularSearchesWithSummary(req.Context(), queryParams.From, queryParams.To, queryParams.Size, clickAnalytics, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing popular searches response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getRecentSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())

		minCharParam := req.URL.Query().Get("min_chars")

		var minChar *int

		if minCharParam != "" {
			minCharAsInt, err := strconv.Atoi(minCharParam)
			if err != nil {
				log.Errorln(logTag, `: invalid "min_chars" value provided`, err)
			} else {
				minChar = &minCharAsInt
			}
		}

		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching recent searches"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		filters := getCustomFilters(req.URL.Query())

		showGlobal := false
		reqCredential, err := credential.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while reading the auth credentials"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		// show_global action is only allowed for user credential
		if reqCredential == credential.User && req.URL.Query().Get("show_global") == "true" {
			showGlobal = true
		}
		// Set the user_id as the IP address of the request if not present in custom events
		if !showGlobal {
			requestIP := extractIPFromRequest(req)
			if filters != nil {
				if filters["user_id"] == nil {
					filters["user_id"] = requestIP
				}
			} else {
				filters = map[string]interface{}{
					"user_id": requestIP,
				}
			}
		}

		raw, err := a.es.recentSearches(req.Context(), queryParams.From, queryParams.To, queryParams.Size, minChar, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing recent searches response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getNoResultSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching no result searches"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		raw, err := a.es.noResultSearchesWithSummary(req.Context(), queryParams.From, queryParams.To, queryParams.Size, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing no result searches response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularFilters() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching popular filters"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		raw, err := a.es.popularFiltersWithSummary(req.Context(), queryParams.From, queryParams.To, queryParams.Size, clickAnalytics, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing popular filters response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularResults() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching popular results"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		// Set the user_id as the IP address of the request if not present in custom events
		if req.URL.Query().Get("show_global") != "true" {
			requestIP := extractIPFromRequest(req)
			if filters != nil {
				if filters["user_id"] == nil {
					filters["user_id"] = requestIP
				}
			} else {
				filters = map[string]interface{}{
					"user_id": requestIP,
				}
			}
		}
		raw, err := a.es.popularResultsWithSummary(req.Context(), queryParams.From, queryParams.To, queryParams.Size, clickAnalytics, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing popular results response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getRecentResults() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching recent results"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		showGlobal := false
		reqCredential, err := credential.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while reading the auth credentials"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		// show_global action is only allowed for user credential
		if reqCredential == credential.User && req.URL.Query().Get("show_global") == "true" {
			showGlobal = true
		}
		// Set the user_id as the IP address of the request if not present in custom events
		if !showGlobal {
			requestIP := extractIPFromRequest(req)
			if filters != nil {
				if filters["user_id"] == nil {
					filters["user_id"] = requestIP
				}
			} else {
				filters = map[string]interface{}{
					"user_id": requestIP,
				}
			}
		}
		raw, err := a.es.recentResults(req.Context(), queryParams.From, queryParams.To, queryParams.Size, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing recent results response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getGeoRequestsDistribution() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching geo requests distribution"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		raw, err := a.es.geoRequestsDistributionWithSummary(req.Context(), queryParams.From, queryParams.To, queryParams.Size, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing geo requests distribution response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getSearchLatencies() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching search latencies"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		raw, err := a.es.latenciesWithSummary(req.Context(), queryParams.From, queryParams.To, queryParams.Size, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing search latencies response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())
		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching analytics summary"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		raw, err := a.es.summary(req.Context(), queryParams.From, queryParams.To, filters, indices...)
		if err != nil {
			msg := "error occurred while parsing analytics summary response"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getRequestDistribution() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := rangeQueryParams(req.URL.Query())

		interval, err := util.IntervalForRange(queryParams.From, queryParams.To)
		if err != nil {
			msg := fmt.Sprintf("invalid query params passed: %v", err)
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching request distribution"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		filters := getCustomFilters(req.URL.Query())
		raw, err := a.es.getRequestDistributionWithSummary(req.Context(), queryParams, interval, queryParams.Size, filters, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getInsights() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName := vars["index"]

		insightResponse, err := a.es.getInsights(req.Context(), indexName, false)
		if err != nil {
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		response := map[string]interface{}{
			"insights": insightResponse.Insights,
			"saved":    insightResponse.SavedInsights,
			"read":     insightResponse.ReadInsights,
		}
		marshalled, _ := json.Marshal(response)

		util.WriteBackRaw(w, marshalled, http.StatusOK)
	}
}

func (a *Analytics) putInsightStatus() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName := vars["index"]

		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var body InsightStatusRequest
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}
		if body.ID == nil {
			util.WriteBackError(w, "Field `id` can not be empty", http.StatusBadRequest)
			return
		}
		docID := getInsightStatusID(indexName)
		err2 := a.es.updateInsightStatus(req.Context(), docID, *body.ID, body)
		if err2 != nil {
			log.Errorln(logTag, ":", err2)
			util.WriteBackError(w, err2.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackMessage(w, "Successfully updated the insight status", http.StatusOK)
	}
}

func (a *Analytics) getFilterLabels() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		raw, err := a.es.getFilterLabels(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getFilterValues() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		prefix := req.URL.Query().Get("prefix")
		vars := mux.Vars(req)
		label, ok := vars["label"]
		if !ok {
			util.WriteBackError(w, `can't get filter values without a "label"`, http.StatusBadRequest)
			return
		}

		indices, err := index.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching filter values"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.getFilterValues(req.Context(), label, prefix, indices...)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) putSavedSearch() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var body SavedSearchRequest
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}
		res := a.RecordSavedSearch(req, body)
		if res.Code != http.StatusOK {
			log.Errorln(logTag, ":", res.Body)
			telemetry.WriteBackErrorWithTelemetry(req, w, res.Body, res.Code)
			return
		} else {
			util.WriteBackRaw(w, []byte(res.Body), http.StatusOK)
		}
	}
}

func (a *Analytics) RecordSavedSearch(req *http.Request, body SavedSearchRequest) rules.ScriptResponse {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json; charset=utf-8"
	headers["X-Content-Type-Options"] = "nosniff"
	if body.QueryId == nil || strings.TrimSpace(*body.QueryId) == "" {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field `query_id` can not be empty",
			Headers: headers,
		}
	}
	if body.SavedSearchId == nil || strings.TrimSpace(*body.SavedSearchId) == "" {
		savedSearchId := uuid.New().String()
		body.SavedSearchId = &savedSearchId
	}
	err2 := a.es.updateSavedSearch(req.Context(), body)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		code := err2.code
		if code == 0 {
			code = http.StatusInternalServerError
		}
		return rules.ScriptResponse{
			Code:    code,
			Body:    err2.message.Error(),
			Headers: headers,
		}
	}
	marshalledResponse, _ := json.Marshal(map[string]interface{}{
		"id": *body.SavedSearchId,
	})
	return rules.ScriptResponse{
		Code:    http.StatusOK,
		Body:    string(marshalledResponse),
		Headers: headers,
	}
}

func (a *Analytics) getSavedSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		customEvents := getCustomFilters(req.URL.Query())
		queryParams := rangeQueryParams(req.URL.Query())
		minCharParam := req.URL.Query().Get("min_chars")

		var minChar *int

		if minCharParam != "" {
			minCharAsInt, err := strconv.Atoi(minCharParam)
			if err != nil {
				log.Errorln(logTag, `: invalid "min_chars" value provided`, err)
			} else {
				minChar = &minCharAsInt
			}
		}
		showGlobal := req.URL.Query().Get("show_global") == "true"
		reqCredential, err := credential.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while reading the auth credentials"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		// show_global action is only allowed for user credential
		if showGlobal && reqCredential != credential.User {
			util.WriteBackError(w, "show_global option is only available for user credentials", http.StatusBadRequest)
			return
		}
		// Set the user_id as the IP address of the request if not present in custom events
		if !showGlobal {
			requestIP := extractIPFromRequest(req)
			if customEvents != nil {
				if customEvents["user_id"] == nil {
					customEvents["user_id"] = requestIP
				}
			} else {
				customEvents = map[string]interface{}{
					"user_id": requestIP,
				}
			}
		}
		filters := SavedSearchesFilters{
			CustomEvents:  customEvents,
			FromTimeStamp: queryParams.From,
			ToTimeStamp:   queryParams.To,
			TimeZone:      queryParams.Timezone,
			Size:          queryParams.Size,
			MinChars:      minChar,
		}
		raw, err := a.es.getSavedSearches(req.Context(), filters)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		marshalled, _ := json.Marshal(raw)
		util.WriteBackRaw(w, marshalled, http.StatusOK)
	}
}

func (a *Analytics) putFavorite() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		reqBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var body FavoriteRequest
		err = json.Unmarshal(reqBody, &body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, "Can't parse request body", http.StatusBadRequest)
			return
		}
		res := a.RecordFavorite(req, body)
		if res.Code != http.StatusOK {
			log.Errorln(logTag, ":", res.Body)
			telemetry.WriteBackErrorWithTelemetry(req, w, res.Body, res.Code)
			return
		} else {
			util.WriteBackRaw(w, []byte(res.Body), http.StatusOK)
		}

	}
}

func (a *Analytics) RecordFavorite(req *http.Request, body FavoriteRequest) rules.ScriptResponse {
	headers := make(map[string]string)
	headers["Content-Type"] = "application/json; charset=utf-8"
	headers["X-Content-Type-Options"] = "nosniff"
	if body.QueryId == nil || strings.TrimSpace(*body.QueryId) == "" {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field `query_id` can not be empty",
			Headers: headers,
		}
	}
	if body.FavoriteOn == nil || strings.TrimSpace(*body.FavoriteOn) == "" {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field `favorite_on` can not be empty",
			Headers: headers,
		}
	}
	if body.Source == nil {
		return rules.ScriptResponse{
			Code:    http.StatusBadRequest,
			Body:    "Field `source` can not be empty",
			Headers: headers,
		}
	}
	if body.Id == nil || strings.TrimSpace(*body.Id) == "" {
		favoriteId := uuid.New().String()
		body.Id = &favoriteId
	}
	err2 := a.es.updateFavorite(req.Context(), body)
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		code := err2.code
		if code == 0 {
			code = http.StatusInternalServerError
		}
		return rules.ScriptResponse{
			Code:    code,
			Body:    err2.message.Error(),
			Headers: headers,
		}
	}
	marshalledResponse, _ := json.Marshal(map[string]interface{}{
		"id": *body.Id,
	})
	return rules.ScriptResponse{
		Code:    http.StatusOK,
		Body:    string(marshalledResponse),
		Headers: headers,
	}
}

func (a *Analytics) getFavorites() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		customEvents := getCustomFilters(req.URL.Query())
		queryParams := rangeQueryParams(req.URL.Query())
		minCharParam := req.URL.Query().Get("min_chars")

		var minChar *int

		if minCharParam != "" {
			minCharAsInt, err := strconv.Atoi(minCharParam)
			if err != nil {
				log.Errorln(logTag, `: invalid "min_chars" value provided`, err)
			} else {
				minChar = &minCharAsInt
			}
		}
		showGlobal := req.URL.Query().Get("show_global") == "true"
		reqCredential, err := credential.FromContext(req.Context())
		if err != nil {
			msg := "error occurred while reading the auth credentials"
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		// show_global action is only allowed for user credential
		if showGlobal && reqCredential != credential.User {
			util.WriteBackError(w, "show_global option is only available for user credentials", http.StatusBadRequest)
			return
		}
		// Set the user_id as the IP address of the request if not present in custom events
		if !showGlobal {
			requestIP := extractIPFromRequest(req)
			if customEvents != nil {
				if customEvents["user_id"] == nil {
					customEvents["user_id"] = requestIP
				}
			} else {
				customEvents = map[string]interface{}{
					"user_id": requestIP,
				}
			}
		}
		filters := SavedSearchesFilters{
			CustomEvents:  customEvents,
			FromTimeStamp: queryParams.From,
			ToTimeStamp:   queryParams.To,
			TimeZone:      queryParams.Timezone,
			Size:          queryParams.Size,
			MinChars:      minChar,
		}
		raw, err := a.es.getFavorites(req.Context(), filters)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		marshalled, _ := json.Marshal(raw)
		util.WriteBackRaw(w, marshalled, http.StatusOK)
	}
}

// saveRecentDocument will save the passed recent document against the
// userId passed.
func (rx *Analytics) saveRecentDocument() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Parse the `index` name from the path.
		vars := mux.Vars(req)
		indexPassed := vars["index"]

		if indexPassed == "" {
			errMsg := "`index` cannot be empty!"
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Read the body and unmarshal into the proper type
		reqBody, err := io.ReadAll(req.Body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "Can't read request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var recentDocumentPassed RecentDocumentPublic
		unmarshalErr := json.Unmarshal(reqBody, &recentDocumentPassed)
		if unmarshalErr != nil {
			errMsg := fmt.Sprint("Error unmarshaling passed body: ", unmarshalErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Inject the `index` into the body
		recentDocumentPassed.Index = &indexPassed

		// If `userId` is not passed, default to the IP address.
		if recentDocumentPassed.UserId == nil {
			userIdParsed := extractIPFromRequest(req)
			recentDocumentPassed.UserId = &userIdParsed
		}

		// Validate the passed body
		validateErr := validateDocument(recentDocumentPassed)
		if validateErr != nil {
			errMsg := fmt.Sprint("Error while validating the recent documents: ", validateErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusBadRequest)
			return
		}

		// Generate the ID of the document
		recentDocumentId := GenerateRecentDocId(*recentDocumentPassed.DocumentId, *recentDocumentPassed.Index)

		// Check if the `documentId` is already present in index, and just do
		// a script update with the userId.
		doesExists, existCheckErr := rx.recentDocuments.doesDocumentExist(req.Context(), recentDocumentId)
		if existCheckErr != nil {
			errMsg := fmt.Sprint("error checking whether or not doc exists: ", existCheckErr.Error())
			log.Errorln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		if doesExists {
			// Update the older one with the new userId and return
			updateErr := rx.recentDocuments.updateRecentDocumentWithUserId(req.Context(), recentDocumentId, *recentDocumentPassed.UserId)
			if updateErr != nil {
				errMsg := fmt.Sprint("error while updating older document Id with new user details: ", updateErr.Error())
				log.Errorln(logTag, ": ", errMsg)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}

			// Update was successful, we can return.
			util.WriteBackMessage(w, "Recent document updated successfully!", http.StatusOK)
			return
		}

		// Inject the `source` if not passed
		sourceFetched, sourceFetchErr := FetchDocumentForSource(*recentDocumentPassed.Index, *recentDocumentPassed.DocumentId)
		if sourceFetchErr != nil {
			errMsg := fmt.Errorf("error fetching source: %s", sourceFetchErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg.Error(), http.StatusInternalServerError)
			return
		}

		recentDocumentPassed.Source = &sourceFetched

		// Convert the passed body into store-able structure and store in ES.
		recentDocumentsToStore := recentDocumentPassed.ToInternal()

		indexErr := rx.recentDocuments.createRecentDocument(req.Context(), recentDocumentsToStore, recentDocumentId)
		if indexErr != nil {
			errMsg := fmt.Errorf("error while indexing the recent document: %s", indexErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg.Error(), http.StatusInternalServerError)
			return
		}

		// Return a success message
		util.WriteBackMessage(w, "Recent document created successfully!", http.StatusOK)
	}
}

// getRecentDocuments will get the recent documents based on the passed filters
func (rx *Analytics) getRecentDocuments() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Parse the query params to filter the result
		queryParams := rangeQueryParams(req.URL.Query())

		// Parse the `user_id` filter if it is passed
		userIdPassed := req.URL.Query().Get("user_id")

		// Parse the `index` filter if it is passed.
		// We will support multiple indices to be passed separate by a
		// comma.
		indexPassed := req.URL.Query().Get("index")
		indicesPassed := make([]string, 0)
		if indexPassed != "" {
			indicesPassed = strings.Split(indexPassed, ",")
		}

		recentDocumentsResponse, recentDocumentsErr := rx.recentDocuments.getRecentDocumentsWithFilter(req.Context(), queryParams, userIdPassed, indicesPassed)
		if recentDocumentsErr != nil {
			errMsg := fmt.Sprint("error fetching recent documents: ", recentDocumentsErr.Error())
			log.Warnln(logTag, ": ", errMsg)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, recentDocumentsResponse, http.StatusOK)
	}
}
