package util

import (
	"encoding/json"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func RandStr() string {
	tokens := strings.Split(uuid.New().String(), "-")
	return tokens[len(tokens)-1]
}

func WriteBackMessage(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	err := json.NewEncoder(w).Encode(map[string]interface{}{"message": msg})
	if err != nil {
		WriteBackError(w, err.Error(), http.StatusInternalServerError)
	}
}

func WriteBackError(w http.ResponseWriter, err string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": err})
}

func WriteBackRaw(w http.ResponseWriter, raw []byte, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	w.Write(raw)
}

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

func WithPrecision(num float64, precision int) float64 {
	output := math.Pow(10, float64(precision))
	return math.Round(num*output) / output
}