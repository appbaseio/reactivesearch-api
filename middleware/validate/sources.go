package validate

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/util"
	"github.com/appbaseio/arc/util/iplookup"
)

const logTag = "[validate]"

// Sources returns a middleware that validates the request sources against the permission sources.
func Sources() middleware.Middleware {
	return sources
}

func sources(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqIP := iplookup.FromRequest(req)
			if reqIP == "" {
				msg := fmt.Sprintf(`failed to recognize request ip: "%s"`, reqIP)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
			ip := net.ParseIP(reqIP)

			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v\n", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			allowedSources := reqPermission.Sources

			var validated bool
			for _, source := range allowedSources {
				if source == "0.0.0.0/0" {
					validated = true
					break
				}
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
				msg := fmt.Sprintf(`permission with username %s doesn't have required sources. reqIP = %s, sources = %s`,
					reqPermission.Username, reqIP, allowedSources)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, req)
	}
}
