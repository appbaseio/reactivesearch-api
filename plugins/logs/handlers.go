package logs

import (
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/util"
)

func (l *Logs) getLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		indices := util.IndicesFromRequest(r)

		from := r.URL.Query().Get("from")
		if from == "" {
			from = "0"
		}
		size := r.URL.Query().Get("size")
		if size == "" {
			size = "100"
		}

		raw, err := l.es.getLogsRaw(from, size, indices...)
		if err != nil {
			log.Printf("%s: error fetching logs: %v\n", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
