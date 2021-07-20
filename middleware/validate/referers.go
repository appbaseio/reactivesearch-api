package validate

import (
	"net/http"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/util"
)

// Referers returns a middleware that validates the request referers against the permission referers.
func Referers() middleware.Middleware {
	return referers
}

func referers(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqDomain := req.Header.Get("Referer")

			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			var validated bool
			for _, referer := range reqPermission.Referers {
				if referer == "*" {
					validated = true
					break
				}
				referer = strings.Replace(referer, "*", ".*", -1)
				matched, err := regexp.MatchString(referer, reqDomain)
				if err != nil {
					log.Errorln(logTag, ":", err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if matched {
					validated = true
					break
				}
			}

			if !validated {
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				util.WriteBackError(w, "permission doesn't have required referers", http.StatusUnauthorized)
				return
			}
		}

		h(w, req)
	}
}
