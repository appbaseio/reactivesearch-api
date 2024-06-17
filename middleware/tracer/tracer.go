package tracer

import (
	"net/http"

	"github.com/appbaseio-confidential/reactivesearch/util"
	httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
)

func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resource := r.RequestURI + "_" + r.Method
		httptrace.TraceAndServe(next, w, r, &httptrace.ServeConfig{
			Service:     util.GetServiceName(),
			Resource:    resource,
			QueryParams: true,
		})
	})
}
