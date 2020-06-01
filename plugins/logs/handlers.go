package logs

import (
	"net/http"
	"net/url"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/util"
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

func (l *Logs) getLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indices := util.IndicesFromRequest(req)

		offset := req.URL.Query().Get("from")
		if offset == "" {
			offset = "0"
		}

		rangeParams := rangeQueryParams(req.URL.Query())

		filter := req.URL.Query().Get("filter")

		raw, err := l.es.getRawLogs(req.Context(), offset, rangeParams.StartDate, rangeParams.EndDate, rangeParams.Size, filter, indices...)
		if err != nil {
			log.Errorln(logTag, ": error fetching logs :", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
