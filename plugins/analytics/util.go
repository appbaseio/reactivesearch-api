package analytics

import (
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultResponseSize = 100
)

// TODO: Make this function to return a map with default values for required query params.
// rangeQueryParams returns the common query params that every analytics endpoint expects,
// - "from": start of the duration in consideration
// - "to"  : end of the duration in consideration
// - "size": no. of response entries
func rangeQueryParams(values url.Values) (from, to string, size int) {
	from, to = previousWeekRange()
	size = 100

	value := values.Get("from")
	if value != "" {
		_, err := time.Parse(time.RFC3339, value)
		if err != nil {
			log.Printf(`%s: unsupported "from" value provided, defaulting to previous week: %v`,
				logTag, err)
		} else {
			from = value
		}
	}

	value = values.Get("to")
	if value != "" {
		_, err := time.Parse(time.RFC3339, value)
		if err != nil {
			log.Printf(`%s: unsupported "to" value provided, defaulting to current time: %v`,
				logTag, err)
		} else {
			to = value
		}
	}

	respSize := values.Get("size")
	if respSize != "" {
		value, err := strconv.Atoi(respSize)
		if err != nil {
			value = defaultResponseSize
			log.Printf(`%s: invalid "size" value provided, defaulting to 100: %v`, logTag, err)
		}
		if value > 1000 {
			value = defaultResponseSize
			log.Printf(`%s: "size" limit exceeded (> 1000), default to 100`, logTag)
		}
		size = value
	}

	return
}

// previousWeekRange returns one week's duration starting from the current instant seven days ago.
func previousWeekRange() (from, to string) {
	now := time.Now()
	from = now.AddDate(0, 0, -7).Format(time.RFC3339)
	to = now.Format(time.RFC3339)
	return
}

// parse splits the comma separated key-value pairs (k1=v1, k2=v3) present in the header.
func parse(header string) []map[string]string {
	var m []map[string]string
	tokens := strings.Split(header, ",")
	for _, token := range tokens {
		values := strings.Split(token, "=")
		if len(values) == 2 {
			m = append(m, map[string]string{
				"key":   strings.TrimSpace(values[0]),
				"value": strings.TrimSpace(values[1]),
			})
		}
	}
	return m
}
