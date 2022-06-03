package nodes

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

type ESNode struct {
	PingTime *int64 `json:"ping_time"`
}

type ArcHealthResponse struct {
	Health         string `json:"health"`
	NodeCount      int64  `json:"node_count"`
	NodeCountSeven int64  `json:"node_count_7d"`
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
	err := n.es.deleteOlderRecords(context.Background())

	if err != nil {
		log.Errorln(logTag, ": error while deleting outdated records, ", err)
	}
}

// StartAutomatedJobs will start all jobs related to nodes
// syncinc and deleting.
//
// This method will start the following jobs in the given
// interval
// - ping job: every 1m
// - delete job: every 7d
func (n *nodes) StartAutomatedJobs() {
	// Start the ping job
	pingESJob := cron.New()
	pingESJob.AddFunc("@every 1m", n.PingESWithTime)
	pingESJob.Start()

	// Start the delete job
	deleteNodeJob := cron.New()
	deleteNodeJob.AddFunc("@every 7d", n.DeleteOutdated)
	deleteNodeJob.Start()
}
