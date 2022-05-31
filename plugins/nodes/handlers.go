package nodes

import (
	"encoding/json"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/util"
)

// healtCheckNodes will return the health status of the node
// along with the number of active nodes in the last 10 mins
// and the last 7 days.
func healtCheckNodes() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		response := map[string]interface{}{
			"health": "ok",
		}

		// Marshal the response
		//
		// NOTE: No need to check error since response is manually created
		// in the above lines
		responseInBytes, _ := json.Marshal(response)
		util.WriteBackRaw(w, responseInBytes, http.StatusOK)
	}
}
