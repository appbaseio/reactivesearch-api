package interceptor

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/appbaseio/arc/errors"
	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/util"
)

const (
	logTag          = "[interceptor]"
	envEsClusterURL = "ES_CLUSTER_URL"
)

// Redirect returns a middleware that redirects the es requests to upstream elasticsearch.
func Redirect() middleware.Middleware {
	return redirect
}

func redirect(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawURL := os.Getenv("ES_CLUSTER_URL")
		if rawURL == "" {
			err := errors.NewEnvVarNotSetError(envEsClusterURL)
			log.Printf("%s: %v", logTag, err)
			return
		}
		esURL, err := url.Parse(rawURL)
		if err != nil {
			log.Printf("%s: error parsing %s=%s: %v", logTag, rawURL, envEsClusterURL, err)
			return
		}

		r.URL.Scheme = esURL.Scheme
		r.URL.Host = esURL.Host
		r.URL.User = esURL.User

		req, err := redirectRequest(r)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		req = req.WithContext(r.Context())

		// disable gzip compression
		encoding := req.Header.Get("Accept-Encoding")
		if encoding != "" {
			req.Header.Set("Accept-Encoding", "identity")
		}

		// set request content type
		v := req.Header.Get("Content-Type")
		if v == "" {
			req.Header.Set("Content-Type", "application/json")
		}

		h(w, req)
	}
}

func redirectRequest(r *http.Request) (*http.Request, error) {
	redirectRequest, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		return nil, err
	}
	redirectRequest.Header = r.Header
	redirectRequest.Header.Del("Authorization")

	// set request content type
	v := redirectRequest.Header.Get("Content-Type")
	if v == "" {
		redirectRequest.Header.Set("Content-Type", "application/json")
	}

	return redirectRequest, nil
}
