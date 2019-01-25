package validate

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/credential"
	"github.com/appbaseio-confidential/arc/model/index"
	"github.com/appbaseio-confidential/arc/model/permission"
	"github.com/appbaseio-confidential/arc/model/user"
	"github.com/appbaseio-confidential/arc/util"
)

func Indices() middleware.Middleware {
	return indices
}

func indices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		errMsg := "an error occurred while validating indices"
		reqIndices, err := index.FromContext(ctx)
		if err != nil {
			log.Printf("%s: unable to fetch indices from request context", logTag)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		reqCredential, err := credential.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, errMsg, http.StatusInternalServerError)
			return
		}

		if len(reqIndices) == 0 {
			// validate cluster level access
			ok, err := allowedClusterAccess(ctx, reqCredential)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !ok {
				util.WriteBackError(w, "credentials cannot access cluster level routes", http.StatusUnauthorized)
				return
			}
		} else {
			// validate index level access
			ok, err := allowedIndexAccess(ctx, reqCredential, reqIndices)
			if err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, errMsg, http.StatusInternalServerError)
				return
			}
			if !ok {
				msg := fmt.Sprintf("credentials cannot access %v index/indices", reqIndices)
				util.WriteBackError(w, msg, http.StatusUnauthorized)
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
