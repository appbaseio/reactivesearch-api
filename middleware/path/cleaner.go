package path

import (
	"net/http"
)

// Clean removes trailing "/"s from the request paths.
func Clean(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = clean(r.URL.Path)
		h(w, r)
	}
}

func clean(path string) string {
	var count int
	for i := len(path) - 1; i > 0; i-- {
		if string(path[i]) == `/` {
			count++
		} else {
			break
		}
	}
	return path[0 : len(path)-count]
}
