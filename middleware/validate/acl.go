package validate

import (
	"context"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
)

// ACL returns a middleware that validates the request acl against the credential acls.
func ACL() middleware.Middleware {
	return validateACL
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method == http.MethodGet {
			if req.RequestURI == "/" {
				h(w, req)
				return
			}
		}
		ctx := req.Context()

		errMsg := "an error occurred while validating request acl"
		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		ok, err := hasACL(ctx, reqCredential, reqACL)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		if !ok {
			msg := fmt.Sprintf(`credentials cannot access "%s" acl`, reqACL.String())
			w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
			return
		}

		h(w, req)
	}
}

func hasACL(ctx context.Context, c credential.Credential, acl *acl.ACL) (bool, error) {
	switch c {
	case credential.User:
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqUser.HasACL(*acl), nil
	case credential.Permission:
		reqPermission, err := permission.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqPermission.HasACL(*acl), nil
	default:
		return false, fmt.Errorf("invalid credentials state reached")
	}
}
