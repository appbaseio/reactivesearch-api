package validate

import (
	"context"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/model/credential"
	"github.com/appbaseio/arc/model/op"
	"github.com/appbaseio/arc/model/permission"
	"github.com/appbaseio/arc/model/user"
	"github.com/appbaseio/arc/util"
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
			log.Error(logTag, ": ", err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Error(logTag, ": ", err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		ok, err := canPerform(ctx, reqCredential, reqOp)
		if err != nil {
			log.Error(logTag, ": ", err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !ok {
			msg := fmt.Sprintf(`credential cannot perform "%v" operation`, reqOp.String())
			util.WriteBackError(w, msg, http.StatusUnauthorized)
			return
		}

		h(w, req)
	}
}

func canPerform(ctx context.Context, c credential.Credential, o *op.Operation) (bool, error) {
	switch c {
	case credential.User:
		reqUser, err := user.FromContext(ctx)
		if err != nil {
			return false, err
		}
		return reqUser.CanDo(*o), nil
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
