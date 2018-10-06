package interceptor

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/appbaseio-confidential/arc/internal/errors"
)

const (
	logTag          = "[interceptor]"
	envEsClusterURL = "ES_CLUSTER_URL"
)

var (
	instance *interceptor
	once     sync.Once
)

type interceptor struct{}

func Instance() *interceptor {
	once.Do(func() {
		instance = &interceptor{}
	})
	return instance
}

// TODO: Create a new request?
func (i *interceptor) Intercept(h http.HandlerFunc) http.HandlerFunc {
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

		// TODO: handle gzip?
		encoding := r.Header.Get("Accept-Encoding")
		if encoding != "" {
			r.Header.Set("Accept-Encoding", "identity")
		}

		v := r.Header.Get("Content-Type")
		if v == "" {
			r.Header.Set("Content-Type", "application/json")
		}
		r.RequestURI = ""

		h(w, r)
	}
}
