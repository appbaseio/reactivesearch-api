package nodes

import (
	"encoding/json"
	"net/http"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// healthCheckNodes will return the health status of the node
// along with the number of active nodes in the last 10 mins
// and the last 7 days.
func (n *nodes) healthCheckNodes() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		response := ArcHealthResponse{
			Health: "ok",
		}

		// Add the node counts after fetching it from ES
		activeTenMins, err := n.es.activeNodesInTenMins(req.Context())
		if err != nil {
			log.Warnln(logTag, ": error while getting the active node count for ten mins, ", err)
			activeTenMins = 0
		}

		response.NodeCount = activeTenMins

		// Add the node count for last 7 days from ES
		activeSevenDays, err := n.es.activeNodesInSevenDays(req.Context())
		if err != nil {
			log.Warnln(logTag, ": error while getting the active node count for seven days, ", err)
			activeSevenDays = 0
		}

		response.NodeCountSeven = activeSevenDays

		// Marshal the response
		//
		// NOTE: No need to check error since response is manually created
		// in the above lines
		responseInBytes, _ := json.Marshal(response)
		util.WriteBackRaw(w, responseInBytes, http.StatusOK)
	}
}
