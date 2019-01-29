package validate

import (
	"fmt"
	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/util"
	"log"
	"net/http"
)

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
