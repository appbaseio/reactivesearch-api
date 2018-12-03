package analytics

import (
	"log"
	"net/http"
	"strconv"

	"github.com/appbaseio-confidential/arc/util"
)

func (a *Analytics) getOverview() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clickAnalytics bool
		q := r.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching analytics overview"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.analyticsOverview(from, to, size, clickAnalytics, indices...)
		if err != nil {
			msg := "error occurred while aggregating analytics overview results"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getAdvanced() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clickAnalytics bool
		q := r.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching advanced analytics"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.advancedAnalytics(from, to, size, clickAnalytics, indices...)
		if err != nil {
			msg := "error occurred while aggregating advanced analytics results"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clickAnalytics bool
		q := r.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching popular searches"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.popularSearchesRaw(from, to, size, clickAnalytics, indices...)
		if err != nil {
			msg := "error occurred while parsing popular searches response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getNoResultSearches() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching no result searches"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.noResultSearchesRaw(from, to, size, indices...)
		if err != nil {
			msg := "error occurred while parsing no result searches response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularFilters() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clickAnalytics bool
		q := r.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching popular filters"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.popularFiltersRaw(from, to, size, clickAnalytics, indices...)
		if err != nil {
			msg := "error occurred while parsing popular filters response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getPopularResults() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var clickAnalytics bool
		q := r.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching popular results"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.popularResultsRaw(from, to, size, clickAnalytics, indices...)
		if err != nil {
			msg := "error occurred while parsing popular results response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getGeoRequestsDistribution() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching geo requests distribution"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.geoRequestsDistribution(from, to, size, indices...)
		if err != nil {
			msg := "error occurred while parsing geo requests distribution response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getSearchLatencies() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, size := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching search latencies"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.latencies(from, to, size, indices...)
		if err != nil {
			msg := "error occurred while parsing search latencies response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		from, to, _ := rangeQueryParams(r.URL.Query())
		indices, err := util.IndicesFromContext(r.Context())
		if err != nil {
			msg := "error occurred while fetching analytics summary"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.summary(from, to, indices...)
		if err != nil {
			msg := "error occurred while parsing analytics summary response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
