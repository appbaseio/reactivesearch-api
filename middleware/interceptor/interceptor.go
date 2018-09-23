package interceptor

import (
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/appbaseio-confidential/arc/arc/middleware/order"
)

type Interceptor struct {
	order.Single
}

func New() Interceptor {
	return Interceptor{}
}

// TODO: Create a new request?
func esInterceptor(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawURL := os.Getenv("ES_CLUSTER_URL")
		if rawURL == "" {
			log.Println("[ERROR]: env var ES_CLUSTER_URL not set")
			return
		}
		esURL, err := url.Parse(rawURL)
		if err != nil {
			log.Printf("[ERROR]: error parsing ES_CLUSTER_URL=%s: %v", rawURL, err)
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
	return i.Adapt(h, esInterceptor)
}
