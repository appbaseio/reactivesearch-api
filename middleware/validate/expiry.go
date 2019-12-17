package validate

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/util"
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
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if reqCredential == credential.Permission {
			reqPermission, err := permission.FromContext(ctx)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			expired, err := reqPermission.IsExpired()
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if expired {
				msg := fmt.Sprintf("permission with username=%s is expired", reqPermission.Username)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, req)
	}
}
