package validate

import (
	"fmt"
	"net"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/appbaseio/reactivesearch-api/util/iplookup"
)

const logTag = "[validate]"

// Sources returns a middleware that validates the request sources against the permission sources.
func Sources() middleware.Middleware {
	return sources
}

// ipv6 and ipv4 addresses when present then it would skip
// the source validation and allow the access
var allowAllAddresses = []string{"::/0", "0000:0000:0000:0000:0000:0000:0000:0000/0", "0.0.0.0/0"}

func sources(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqIP := iplookup.FromRequest(req)
			if reqIP == "" {
				msg := fmt.Sprintf(`failed to recognize request ip: "%s"`, reqIP)
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
				return
			}
			ip := net.ParseIP(reqIP)

			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}
			allowedSources := reqPermission.Sources

			var validated bool
			for _, source := range allowedSources {
				if util.Contains(allowAllAddresses, source) {
					validated = true
					break
				}
				_, ipNet, err := net.ParseCIDR(source)
				if err != nil {
					log.Errorln(logTag, ":", err)
					telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
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
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
				return
			}
		} else {
			reqUser, err := user.FromContext(ctx)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
				return
			}

			if reqUser.Sources != nil {
				// handle user credential
				var allowedSources = *reqUser.Sources

				reqIP := iplookup.FromRequest(req)
				if reqIP == "" {
					msg := fmt.Sprintf(`failed to recognize request ip: "%s"`, reqIP)
					w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
					telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
					return
				}
				ip := net.ParseIP(reqIP)

				var validated bool
				for _, source := range allowedSources {
					if source == "0.0.0.0/0" {
						validated = true
						break
					}
					_, ipNet, err := net.ParseCIDR(source)
					if err != nil {
						log.Errorln(logTag, ":", err)
						telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusInternalServerError)
						return
					}
					if ipNet.Contains(ip) {
						validated = true
						break
					}
				}

				if !validated {
					msg := fmt.Sprintf(`credentials with username %s doesn't have required sources. reqIP = %s, sources = %s`,
						reqUser.Username, reqIP, allowedSources)
					w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
					telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
					return
				}
			}

		}

		h(w, req)
	}
}
