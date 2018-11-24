package referers

import (
	"log"
	"net/http"
	"regexp"

	"github.com/appbaseio-confidential/arc/internal/types/credential"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/util"
)

const logTag = "[referers]"

func Validate(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqDomain := r.Header.Get("Referer")
			if reqDomain == "" {
				util.WriteBackError(w, "failed to identify request domain, empty header: Referer", http.StatusUnauthorized)
				return
			}

			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			allowedReferers := reqPermission.Referers

			var validated bool
			for _, referer := range allowedReferers {
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

		h(w, r)
	}
}
