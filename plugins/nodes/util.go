package nodes

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type ESNode struct {
	PingTime *int64 `json:"ping_time"`
}

// PingESWithTime will ping ES with the timestamp
// and the machine ID of the current node.
//
// It will also setup everything else like getting the
// ES instance etc.
func (n *nodes) PingESWithTime() {
	// Get the machineID
	machineID := util.MachineID

	err := n.es.pingES(context.Background(), machineID)

	if err != nil {
		log.Errorln(logTag, ": error occurred while pinging ES to update time, ", err)
	}
}

// DeleteOutdated will delete all the docs in the index
// that are older than 7 days.
func (n *nodes) DeleteOutdated() {

}
