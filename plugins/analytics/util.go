package analytics

import (
	"log"
	"net/url"
	"strconv"
	"time"
)

func queryParams(values url.Values) (from, to string, size int) {
	from, to = getWeekRange()
	size = 100

	value := values.Get("from")
	if value != "" {
		// TODO: check if supported date format is passed
		from = value
	}

	value = values.Get("to")
	if value != "" {
		// TODO: check if supported date format is passed
		to = value
	}

	respSize := values.Get("size")
	if respSize != "" {
		value, err := strconv.Atoi(respSize)
		if err != nil {
			value = 100
			log.Printf(`%s: invalid "size" value provided, defaulting to 100: %v`, logTag, err)
		}
		if value > 1000 {
			value = 100
			log.Printf(`%s: "size" limit exceeded (> 1000), default to 100`, logTag)
		}
		size = value
	}

	return
}

func getWeekRange() (from, to string) {
	format := "2006/01/02 15:04:05"
	now := time.Now()
	from = now.AddDate(0, 0, -7).Format(format)
	to = now.Format(format)
	return
}
