package validate

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/util"
)

// PermissionExpiry returns a middleware that checks whether a permission is expired or not.
func PermissionExpiry() middleware.Middleware {
	return validateExpiry
}

func validateExpiry(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			expired, err := reqPermission.IsExpired()
			if err != nil {
				log.Errorln(logTag, ":", err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if expired {
				msg := fmt.Sprintf("permission with username=%s is expired", reqPermission.Username)
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, req)
	}
}
