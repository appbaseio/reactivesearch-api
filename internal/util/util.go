package util

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/appbaseio-confidential/arc/internal/types/index"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// RandStr returns "node" field of a UUID.
// See: https://tools.ietf.org/html/rfc4122#section-4.1.6
func RandStr() string {
	tokens := strings.Split(uuid.New().String(), "-")
	return tokens[len(tokens)-1]
}

// WriteBackMessage writes the given message as a json response to the response writer.
func WriteBackMessage(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	msg := map[string]interface{}{
		"code":    code,
		"status":  http.StatusText(code),
		"message": message,
	}
	err := json.NewEncoder(w).Encode(msg)
	if err != nil {
		WriteBackError(w, err.Error(), http.StatusInternalServerError)
	}
}

// WriteBackError writes the given error message as a json response to the response writer.
func WriteBackError(w http.ResponseWriter, err string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	msg := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"status":  http.StatusText(code),
			"message": err,
		},
	}
	json.NewEncoder(w).Encode(msg)
}

// WriteBackRaw writes the given json encoded bytes to the response writer.
func WriteBackRaw(w http.ResponseWriter, raw []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	w.Write(raw)
}

// Contains checks the presence of a string in the given string slice.
func Contains(slice []string, val string) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// DaysInMonth returns the number of days in a month for a given year.
func DaysInMonth(m time.Month, year int) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// DaysInYear returns the number of days in a given year.
func DaysInYear(year int) int {
	return time.Date(year, 0, 0, 0, 0, 0, 0, time.UTC).Day()
}

// DaysInCurrentYear returns the number of days in the current year.
func DaysInCurrentYear() int {
	return DaysInYear(time.Now().Year())
}

// WithPrecision returns the floating point number with the given precision.
func WithPrecision(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return math.Round(num*output) / output
}

// IndicesFromRequest extracts index patterns from the request url (from var "{index}").
func IndicesFromRequest(r *http.Request) ([]string, bool) {
	vars := mux.Vars(r)
	indexVar, ok := vars["index"]
	if !ok {
		return nil, false
	}

	var indices []string
	tokens := strings.Split(indexVar, ",")
	for _, pattern := range tokens {
		pattern = strings.TrimSpace(pattern)
		indices = append(indices, pattern)
	}

	return indices, true
}

// IndicesFromContext fetches index patterns from the request context.
func IndicesFromContext(ctx context.Context) ([]string, error) {
	ctxIndices := ctx.Value(index.CtxKey)
	if ctxIndices == nil {
		return nil, fmt.Errorf("cannot fetch indices from request context")
	}
	indices, ok := ctxIndices.([]string)
	if !ok {
		return nil, fmt.Errorf("cannot cast ctxIndices to []string")
	}
	return indices, nil
}

// CountComponents returns the numbers of "/" and "vars" present in the route.
func CountComponents(route string) (int, int) {
	pattern := `^{.*}$`
	var vars []string

	fragments := strings.Split(route, "/")
	for _, fragment := range fragments {
		matched, _ := regexp.MatchString(pattern, fragment)
		if matched {
			vars = append(vars, fragment)
		}
	}

	return strings.Count(route, "/"), len(vars)

}

var (
	client *http.Client
	once   sync.Once
)

// HTTPClient returns an http client with reasonable timeout defaults.
// See: https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779
// This client will cap the TCP connect and TLS handshake timeouts,
// as well as establishing an end-to-end request timeout.
func HTTPClient() *http.Client {
	once.Do(func() {
		var netTransport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 5 * time.Second,
		}
		var netClient = &http.Client{
			Timeout:   time.Second * 10,
			Transport: netTransport,
		}
		client = netClient
	})
	return client
}
