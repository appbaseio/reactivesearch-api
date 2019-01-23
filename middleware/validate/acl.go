package validate

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/acl"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/util"
)

func ACL() middleware.Middleware {
	return validateACL
}

func validateACL(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		errMsg := "an error occurred while validating request acl"
		reqACL, err := acl.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		ok, err := hasACL(ctx, reqCredential, reqACL)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if !ok {
			msg := fmt.Sprintf(`credentials cannot access "%s" acl`, reqACL.String())
			util.WriteBackError(w, msg, http.StatusUnauthorized)
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
