package analytics

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func rangeQueryParams(values url.Values) (from, to string, size int) {
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

func getIndices(r *http.Request) ([]string, bool) {
	vars := mux.Vars(r)
	indexVar, ok := vars["index"]
	if !ok {
		return nil, false
	}

	var indices []string
	tokens := strings.Split(indexVar, ",")
	for _, index := range tokens {
		index = strings.TrimSpace(index)
		indices = append(indices, index)
	}

	return indices, true
}
