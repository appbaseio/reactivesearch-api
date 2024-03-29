package validate

import (
	"context"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/index"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
)

// Indices returns a middleware that validates the request indices against the credential indices.
func Indices() middleware.Middleware {
	return indices
}

func indices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		errMsg := "an error occurred while validating indices"
		reqIndices, err := index.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ": unable to fetch indices from request context:", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		if len(reqIndices) == 0 {
			// validate cluster level access
			ok, err := allowedClusterAccess(ctx, reqCredential)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}
			if !ok {
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, "credentials cannot access cluster level routes", http.StatusUnauthorized)
				return
			}
		} else {
			// validate index level access
			ok, err := allowedIndexAccess(ctx, reqCredential, reqIndices)
			if err != nil {
				log.Errorln(logTag, ":", err)
				telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
				return
			}
			if !ok {
				msg := fmt.Sprintf("credentials cannot access %v index/indices", reqIndices)
				w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
				telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
				return
			}
		}

		h(w, req)
	}
}

func allowedClusterAccess(ctx context.Context, c credential.Credential) (bool, error) {
	switch c {
	case credential.User:
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqUser.CanAccessCluster()
	case credential.Permission:
		reqPermission, err := permission.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqPermission.CanAccessCluster()
	default:
		return false, fmt.Errorf("illegal credential state reached")
	}
}

func allowedIndexAccess(ctx context.Context, c credential.Credential, indices []string) (bool, error) {
	switch c {
	case credential.User:
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqUser.CanAccessIndices(indices...)
	case credential.Permission:
		reqPermission, err := permission.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqPermission.CanAccessIndices(indices...)
	default:
		return false, fmt.Errorf("illegal credential state reached")
	}
}
