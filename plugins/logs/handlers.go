package logs

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/util"
)

const (
	defaultResponseSize = 100
	defaultTimeFormat   = "2006/01/02"
)

// NormalizedQueryParams represents the normalized query parameters
type NormalizedQueryParams struct {
	StartDate string
	EndDate   string
	Size      int
}

// previousMonthRange returns one month's duration starting from the current instant 30 days ago.
func previousMonthRange() (from, to string) {
	now := time.Now()
	from = now.AddDate(0, 0, -30).Format(time.RFC3339)
	to = now.Format(time.RFC3339)
	return
}

// rangeQueryParams returns the common query params that every analytics endpoint expects,
// - "start_date": start of the duration in consideration
// - "end_date"  : end of the duration in consideration
// - "size": no. of response entries
func rangeQueryParams(values url.Values) NormalizedQueryParams {
	from, to := previousMonthRange()
	size := 100

	value := values.Get("start_date")
	if value != "" {
		t, err := time.Parse(defaultTimeFormat, value)
		if err != nil {
			log.Errorln(logTag, `: unsupported "start_date" value provided, defaulting to previous month:`, err)
		} else {
			from = t.Format(time.RFC3339)
		}
	}

	value = values.Get("end_date")
	if value != "" {
		t, err := time.Parse(defaultTimeFormat, value)
		if err != nil {
			log.Errorln(logTag, `: unsupported "end_date" value provided, defaulting to current time:`, err)
		} else {
			// Use end of the day for to range
			year, month, day := t.Date()
			to = time.Date(year, month, day, 23, 59, 59, 0, t.Location()).Format(time.RFC3339)
		}
	}

	respSize := values.Get("size")
	if respSize != "" {
		value, err := strconv.Atoi(respSize)
		if err != nil {
			value = defaultResponseSize
			log.Errorln(logTag, `: invalid "size" value provided, defaulting to 100:`, err)
		}
		if value > 1000 {
			value = defaultResponseSize
			log.Println(logTag, `: "size" limit exceeded (> 1000), default to 100`)
		}
		size = value
	}

	return NormalizedQueryParams{
		StartDate: from,
		EndDate:   to,
		Size:      size,
	}
}

func (l *Logs) logsHandler(w http.ResponseWriter, req *http.Request, isSearchLogs bool) {
	indices := util.IndicesFromRequest(req)

	offset := req.URL.Query().Get("from")
	if offset == "" {
		offset = "0"
	}

	parsedOffset, err := strconv.Atoi(offset)
	if err != nil {
		errMsg := fmt.Errorf(`invalid value "%v" for query param "from"`, offset)
		log.Errorln(logTag, ": ", errMsg)
		util.WriteBackError(w, err.Error(), http.StatusBadRequest)
		return
	}

	rangeParams := rangeQueryParams(req.URL.Query())

	filter := req.URL.Query().Get("filter")

	logsFilterConfig := logsFilter{
		Offset:    parsedOffset,
		StartDate: rangeParams.StartDate,
		EndDate:   rangeParams.EndDate,
		Size:      rangeParams.Size,
		Filter:    filter,
		Indices:   indices,
	}

	// Apply Search request filters
	if isSearchLogs {
		startLatency := req.URL.Query().Get("start_latency")
		if startLatency != "" {
			startLatencyAsInt, err := strconv.Atoi(startLatency)
			if err != nil {
				errMsg := fmt.Errorf(`invalid value "%v" for query param "start_latency"`, startLatency)
				log.Errorln(logTag, ": ", errMsg)
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}
			logsFilterConfig.StartLatency = &startLatencyAsInt
		}
		endLatency := req.URL.Query().Get("end_latency")
		if endLatency != "" {
			endLatencyAsInt, err := strconv.Atoi(endLatency)
			if err != nil {
				errMsg := fmt.Errorf(`invalid value "%v" for query param "end_latency"`, endLatency)
				log.Errorln(logTag, ": ", errMsg)
				util.WriteBackError(w, errMsg.Error(), http.StatusBadRequest)
				return
			}
			logsFilterConfig.EndLatency = &endLatencyAsInt
		}
		orderBy := req.URL.Query().Get("order_by_latency")
		if orderBy != "" {
			if !(orderBy == "asc" || orderBy == "desc") {
				errMsg := fmt.Errorf(`invalid value "%v" for query param "order_by_latency"`, orderBy)
				log.Errorln(logTag, ": ", errMsg)
				util.WriteBackError(w, errMsg.Error(), http.StatusBadRequest)
				return
			}
			logsFilterConfig.OrderByLatency = orderBy
		} else {
			// If not defined set default order_by value to `desc`
			logsFilterConfig.OrderByLatency = "desc"
		}
		// Use search filter to always get search requests
		logsFilterConfig.Filter = "search"
	}

	raw, err := l.es.getRawLogs(req.Context(), logsFilterConfig)
	if err != nil {
		log.Errorln(logTag, ": error fetching logs :", err)
		util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	util.WriteBackRaw(w, raw, http.StatusOK)
}

func (l *Logs) getLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		l.logsHandler(w, req, false)
	}
}

func (l *Logs) getSearchLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		l.logsHandler(w, req, true)
	}
}
