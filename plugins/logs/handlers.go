package logs

import (
	"log"
	"net/http"

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

		raw, err := l.es.getRawLogs(req.Context(), from, size, indices...)
		if err != nil {
			log.Printf("%s: error fetching logs: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
