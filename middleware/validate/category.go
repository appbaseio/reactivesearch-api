package validate

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
)

// Category returns a middleware that validates the request category against credential categories.
func Category() middleware.Middleware {
	return validateCategory
}

func validateCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// ignore the category validation for following routes
		// GET /_user
		// GET /
		if req.Method == http.MethodGet {
			if req.RequestURI == "/" || strings.HasSuffix(req.RequestURI, "_user") {
				h(w, req)
				return
			}
		}
		ctx := req.Context()

		errMsg := "an error occurred while validating request category"
		reqCategory, err := category.FromContext(ctx)
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

		ok, err := hasCategory(ctx, reqCredential, reqCategory)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		if !ok {
			msg := fmt.Sprintf(`credential can't access "%s" category`, reqCategory.String())
			w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
			return
		}

		h(w, req)
	}
}

func hasCategory(ctx context.Context, c credential.Credential, cat *category.Category) (bool, error) {
	switch c {
	case credential.User:
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqUser.HasCategory(*cat), nil
	case credential.Permission:
		reqPermission, err := permission.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqPermission.HasCategory(*cat), nil
	default:
		return false, fmt.Errorf("invalid credentials state reached")
	}
}
