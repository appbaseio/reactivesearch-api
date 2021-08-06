package querytranslate

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/ratelimiter"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/op"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	log "github.com/sirupsen/logrus"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(mw []middleware.Middleware, h http.HandlerFunc) http.HandlerFunc {
	// Append query translate middleware at the end
	mw = append(mw, queryTranslate)
	// Append telemetry at the end
	mw = append(mw, telemetry.Recorder())
	return c.Adapt(h, append(list(), mw...)...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classifyOp,
		classify.Indices(),
		saveRequestToCtx, // middleware to save the request body in context
		logs.Recorder(),
		auth.BasicAuth(),
		ratelimiter.Limit(),
		validate.Sources(),
		validate.Referers(),
		validate.Indices(),
		validate.Category(),
		validate.Operation(),
		validate.PermissionExpiry(),
		applySourceFiltering,
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.ReactiveSearch

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyOp(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		// search requests are are a read operation
		operation := op.Read
		ctx := op.NewContext(req.Context(), &operation)
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func saveRequestToCtx(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		var body RSQuery
		err := json.NewDecoder(req.Body).Decode(&body)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, fmt.Sprintf("Can't parse request body: %v", err), http.StatusBadRequest)
			return
		}
		// Set request body as nil to avoid memory issues (storage duplication)
		req.Body = nil
		ctx := NewContext(req.Context(), body)
		req = req.WithContext(ctx)
		h(w, req)
	}
}

func applySourceFiltering(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()
		// Apply source filters
		reqPermission, err := permission.FromContext(ctx)
		if err != nil {
			log.Warnln(logTag, ":", err)
			h(w, req)
			return
		}
		isExcludesPresent := len(reqPermission.Excludes) != 0
		isEmpty := len(reqPermission.Includes) == 0 && len(reqPermission.Excludes) == 0
		isDefaultInclude := len(reqPermission.Includes) > 0 && reqPermission.Includes[0] == "*"
		shouldApplyFilters := !isEmpty && (!isDefaultInclude || isExcludesPresent)
		if shouldApplyFilters {
			requestQuery, err := FromContext(req.Context())
			if err != nil {
				log.Errorln(logTag, ":", err)

				telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
				return
			}
			for index := range requestQuery.Query {
				requestQuery.Query[index].IncludeFields = &reqPermission.Includes
				requestQuery.Query[index].ExcludeFields = &reqPermission.Excludes
			}
		}
		h(w, req)
	}
}

// Translates the query to `_msearch` request
func queryTranslate(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}

		// Translate query
		msearchQuery, err := translateQuery(*body)
		// log.Println("RS QUERY", msearchQuery)
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, err.Error(), http.StatusBadRequest)
			return
		}
		// Update the request body to the parsed query
		req.Body = ioutil.NopCloser(strings.NewReader(msearchQuery))

		// Track plugin
		ctxTrackPlugin := trackplugin.TrackPlugin(req.Context(), "qt")
		req = req.WithContext(ctxTrackPlugin)

		h(w, req)
	}
}
