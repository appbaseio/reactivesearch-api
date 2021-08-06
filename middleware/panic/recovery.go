package panic

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
)

// Recovery is a middleware that wraps an http handler to recover from panics.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var err error
		defer func() {
			r := recover()
			if r != nil {
				switch t := r.(type) {
				case string:
					err = errors.New(t)
				case error:
					err = t
				default:
					err = fmt.Errorf("unknown error occurred: %v", err)
				}
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
		}()
		next.ServeHTTP(w, req)
	})
}
