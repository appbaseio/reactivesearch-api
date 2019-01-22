package analytics

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/appbaseio-confidential/arc/util"
)

func (a *Analytics) getOverview() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching analytics overview"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.analyticsOverview(req.Context(), from, to, size, clickAnalytics, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching advanced analytics"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.advancedAnalytics(req.Context(), from, to, size, clickAnalytics, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching popular searches"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.popularSearchesRaw(req.Context(), from, to, size, clickAnalytics, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching no result searches"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.noResultSearchesRaw(req.Context(), from, to, size, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching popular filters"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.popularFiltersRaw(req.Context(), from, to, size, clickAnalytics, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		var clickAnalytics bool
		q := req.URL.Query().Get("click_analytics")
		if v, err := strconv.ParseBool(q); err == nil {
			clickAnalytics = v
		}
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching popular results"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.popularResultsRaw(req.Context(), from, to, size, clickAnalytics, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching geo requests distribution"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.geoRequestsDistribution(req.Context(), from, to, size, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		from, to, size := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching search latencies"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.latencies(req.Context(), from, to, size, indices...)
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
	return func(w http.ResponseWriter, req *http.Request) {
		from, to, _ := rangeQueryParams(req.URL.Query())
		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching analytics summary"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.summary(req.Context(), from, to, indices...)
		if err != nil {
			msg := "error occurred while parsing analytics summary response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (a *Analytics) getRequestDistribution() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		from, to, size := rangeQueryParams(req.URL.Query())

		interval, err := util.IntervalForRange(from, to)
		if err != nil {
			msg := fmt.Sprintf("invalid query params passed: %v", err)
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, msg, http.StatusBadRequest)
			return
		}

		indices, err := util.IndicesFromContext(req.Context())
		if err != nil {
			msg := "error occurred while fetching request distribution"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}

		raw, err := a.es.getRequestDistribution(req.Context(), from, to, interval, size, indices...)
		if err != nil {
			msg := "error occurred while parsing request distribution response"
			log.Printf("%s: %v\n", logTag, err)
			util.WriteBackError(w, msg, http.StatusInternalServerError)
			return
		}
		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}
