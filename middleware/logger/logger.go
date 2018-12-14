package logger

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const logTag = "[logger]"

// Log logs and records time taken by each requests. As a side effect,
// it trims the railing slashes from the matched route.
func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		r.URL.Path = trimTrailingSlashes(r.URL.Path)
		next.ServeHTTP(w, r)
		log.Println(fmt.Sprintf("%s: finished %s, took %fs",
			logTag, fmt.Sprintf("%s %s", r.Method, r.URL.Path), time.Since(start).Seconds()))
	})
}

func trimTrailingSlashes(path string) string {
	if path != "/" {
		for strings.HasSuffix(path, "/") {
			path = strings.TrimSuffix(path, "/")
		}
	}
	return path
}
