package analytics

import (
	"encoding/json"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	timeFormat          = "2006/01/02 15:04:05"
	defaultResponseSize = 100
)

// rangeQueryParams returns the common query params that every analytics endpoint expects,
// - "from": start of the duration in consideration
// - "to"  : end of the duration in consideration
// - "size": no. of response entries
func rangeQueryParams(values url.Values) (from, to string, size int) {
	from, to = previousWeekRange()
	size = 100

	value := values.Get("from")
	if value != "" {
		_, err := time.Parse(timeFormat, value)
		if err != nil {
			log.Printf(`%s: unsupported "from" value provided, defaulting to previous week: %v`,
				logTag, err)
		} else {
			from = value
		}
	}

	value = values.Get("to")
	if value != "" {
		_, err := time.Parse(timeFormat, value)
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
	from = now.AddDate(0, 0, -7).Format(timeFormat)
	to = now.Format(timeFormat)
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
				"key":   values[0],
				"value": values[1],
			})
		}
	}
	return m
}

func logRaw(record map[string]interface{}) {
	rawRecord, err := json.Marshal(record)
	if err != nil {
		log.Printf("%s: error marshalling analytics record: %v", logTag, err)
	}
	log.Printf("%s: %s", logTag, string(rawRecord))

}
