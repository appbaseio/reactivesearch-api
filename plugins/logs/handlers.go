package logs

import (
	"net/http"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/arc/util"
)

func (l *Logs) getLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indices := util.IndicesFromRequest(req)

		from := req.URL.Query().Get("from")
		if from == "" {
			from = "0"
		}
		size := req.URL.Query().Get("size")
		if size == "" {
			size = "100"
		}

		filter := req.URL.Query().Get("filter")

		raw, err := l.es.getRawLogs(req.Context(), from, size, filter, indices...)
		if err != nil {
			log.Errorln(logTag, ": error fetching logs :", err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
