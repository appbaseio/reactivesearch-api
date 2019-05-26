package validate

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/appbaseio-confidential/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/util"
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
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqDomain := req.Header.Get("Referer")

			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
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
					log.Printf("%s: %v\n", logTag, err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if matched {
					validated = true
					break
				}
			}

			if !validated {
				util.WriteBackError(w, "permission doeesn't have required referers", http.StatusInternalServerError)
				return
			}
		}

		h(w, req)
	}
}
