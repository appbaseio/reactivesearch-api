package sources

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/iplookup"
	"github.com/appbaseio-confidential/arc/internal/types/credential"
	"github.com/appbaseio-confidential/arc/internal/types/permission"
	"github.com/appbaseio-confidential/arc/internal/util"
)

const logTag = "[sources]"

func Validate(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		reqIP := iplookup.FromRequest(r)
		if reqIP == "" {
			msg := fmt.Sprintf(`failed to recognise request ip "%s"`, reqIP)
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}
		ip := net.ParseIP(reqIP)

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			allowedSources := reqPermission.Sources

			var validated bool
			for _, source := range allowedSources {
				_, ipNet, err := net.ParseCIDR(source)
				if err != nil {
					log.Printf("%s: %v\n", logTag, err)
					util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if ipNet.Contains(ip) {
					validated = true
					break
				}
			}

			if !validated {
				msg := fmt.Sprintf(`permission with username "%s" doesn't have required sources`,
					reqPermission.Username)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, r)
	}
}
