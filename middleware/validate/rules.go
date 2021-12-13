package validate

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/model/acl"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
)

// Check if a request is of indexing type
func IndexingRequest() middleware.Middleware {
	return IsIndexingRequest
}

// Check if a request is of indexing type and accordingly
// invoke a middleware.
// We need to check if the request is of indexing type.
// This is done by checking the category to be of type "docs"
// and ACL's should be one of:
// ['index', 'update', 'update_by_query', 'create', 'bulk', 'delete' 'delete_by_query']
//
// If it "is" an indexing request, then the proper method
// will be called to invoke indexing rules.
func IsIndexingRequest(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// Check if the request being passed is an indexing
		// request.
		// We will check that by checking if the category is
		// set to docs.
		// If it is set to docs, then the acl should be one
		// of the validTypes.

		// Declare an error Message
		errMsg := "an error occurred while checking the %s"

		ctx := req.Context()

		reqCategory, err := category.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf(errMsg, "category"), http.StatusInternalServerError)
			return
		}

		// If the category is not docs, just return
		if *reqCategory != category.Docs {
			h(w, req)
			return
		}

		// Check if the ACL matches.
		reqAcl, err := acl.FromContext(ctx)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf(errMsg, "acl"), http.StatusInternalServerError)
			return
		}

		// Check if the ACL is valid for indexing
		if isValidACLForIndexing(reqAcl) {
			println("Valid ACL for indexing passed", reqAcl.String())
		}

		h(w, req)
	}
}

// Check if the passed ACL is a valid value from the list
// of the possible ACL's for an indexing request.
//
// We will just check if the value passed is present
// in a predefined list.
func isValidACLForIndexing(reqACL *acl.ACL) bool {
	reqACLValue := *reqACL
	switch reqACLValue {
	case
		acl.Index,
		acl.Update,
		acl.UpdateByQuery,
		acl.Create,
		acl.Bulk,
		acl.Delete,
		acl.DeleteByQuery:
		return true
	}
	return false
}
