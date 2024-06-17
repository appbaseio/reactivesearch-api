package tracktime

import (
	"net/http"
)

// Tracks the starting time for request
func Track(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := NewTimeTrackerContext(req.Context())
		req = req.WithContext(ctx)
		h.ServeHTTP(w, req)
	})
}
