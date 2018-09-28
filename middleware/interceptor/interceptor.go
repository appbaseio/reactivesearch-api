package interceptor

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/internal/errors"
)

const (
	logTag          = "[interceptor]"
	envEsClusterURL = "ES_CLUSTER_URL"
)

type Interceptor struct {
	order.Single
}

func New() Interceptor {
	return Interceptor{}
}

// TODO: Create a new request?
func (i *Interceptor) intercept(h http.HandlerFunc) http.HandlerFunc {
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

		v := r.Header.Get("Content-Type")
		if v == "" {
			r.Header.Set("Content-Type", "application/json")
		}
		r.RequestURI = ""

		h(w, r)
	}
}

func (i *Interceptor) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return i.Adapt(h, i.intercept)
}
