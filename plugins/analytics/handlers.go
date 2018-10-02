package analytics

import (
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/internal/util"
)

func (a *Analytics) getLatency() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, size := queryParams(r.URL.Query())

		raw, err := a.es.getRawLatency(from, to, size)
		if err != nil {
			log.Printf("%s: error fetching latency: %v", logTag, err)
			util.WriteBackError(w, "error fetching latency", http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
