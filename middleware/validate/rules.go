package validate

import (
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware"
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

		// Check if the category is of type docs and accordingly
		// check the ACL's as well.
		if *reqCategory == category.Docs {
			println("Category is docs")
		}

		h(w, req)
	}
}
