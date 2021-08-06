package validate

import (
	"context"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
)

// Operation returns a middleware that validates the request operation against the credential operations.
func Operation() middleware.Middleware {
	return operation
}

func operation(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		errMsg := "an error occurred while validating request op"
		reqOp, err := op.FromContext(ctx)
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

		ok, err := canPerform(ctx, reqCredential, reqOp)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, errMsg, http.StatusInternalServerError)
			return
		}

		if !ok {
			msg := fmt.Sprintf(`credential cannot perform "%v" operation`, reqOp.String())
			w.Header().Set("www-authenticate", "Basic realm=\"Authentication Required\"")
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusUnauthorized)
			return
		}

		h(w, req)
	}
}

func canPerform(ctx context.Context, c credential.Credential, o *op.Operation) (bool, error) {
	switch c {
	case credential.User:
		// access types (read, write) doesn't matter for user credential
		return true, nil
	case credential.Permission:
		reqPermission, err := permission.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqPermission.CanDo(*o), nil
	default:
		return false, fmt.Errorf("invalid credential state reached")
	}
}
