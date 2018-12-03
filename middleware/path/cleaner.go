package path

import (
	"net/http"
	"strings"
)

// Clean removes trailing "/"s from the request paths.
func Clean(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = removeTrailingSlashes(r.URL.Path)
		h(w, r)
	}
}

func removeTrailingSlashes(path string) string {
	for strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}
