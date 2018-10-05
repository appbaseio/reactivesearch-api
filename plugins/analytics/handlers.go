package analytics

import (
	"log"
	"net/http"
	"strconv"

	"github.com/appbaseio-confidential/arc/internal/util"
)

func (a *Analytics) getOverview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("click_analytics")

		var clickAnalytics bool
		if q != "" {
			if v, err := strconv.ParseBool(q); err == nil {
				clickAnalytics = v
			}
		}
		from, to, size := queryParams(r.URL.Query())

		raw, err := a.es.analyticsOverview(from, to, size, clickAnalytics)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getAdvanced() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("click_analytics")

		var clickAnalytics bool
		if q != "" {
			if v, err := strconv.ParseBool(q); err == nil {
				clickAnalytics = v
			}
		}
		from, to, size := queryParams(r.URL.Query())

		raw, err := a.es.advancedAnalytics(from, to, size, clickAnalytics)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("click_analytics")

		var clickAnalytics bool
		if q != "" {
			if v, err := strconv.ParseBool(q); err == nil {
				clickAnalytics = v
			}
		}
		from, to, size := queryParams(r.URL.Query())

		raw, err := a.es.popularSearches(from, to, size, clickAnalytics)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularFilters() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("click_analytics")

		var clickAnalytics bool
		if q != "" {
			if v, err := strconv.ParseBool(q); err == nil {
				clickAnalytics = v
			}
		}
		from, to, size := queryParams(r.URL.Query())

		raw, err := a.es.popularFilters(from, to, size, clickAnalytics)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularResults() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("click_analytics")

		var clickAnalytics bool
		if q != "" {
			if v, err := strconv.ParseBool(q); err == nil {
				clickAnalytics = v
			}
		}
		from, to, size := queryParams(r.URL.Query())

		raw, err := a.es.popularResults(from, to, size, clickAnalytics)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getNoResultsSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := a.es.noResultsSearches(queryParams(r.URL.Query()))
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getGeoRequestsDistribution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := a.es.geoRequestsDistribution(queryParams(r.URL.Query()))
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getLatencies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := a.es.latencies(queryParams(r.URL.Query()))
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := a.es.summary(queryParams(r.URL.Query()))
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackMessage(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
