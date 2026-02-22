package cache

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/middleware/validate"
	"github.com/appbaseio/reactivesearch-api/model/category"
	"github.com/appbaseio/reactivesearch-api/model/trackplugin"
	"github.com/appbaseio/reactivesearch-api/plugins/auth"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
	"github.com/appbaseio/reactivesearch-api/plugins/querytranslate"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// A list of plans for which this feature will be available
var validPlans = []util.Plan{
	util.ArcEnterprise,
	util.HostedArcEnterprise,
	util.ProductionFirst2019,
	util.ProductionSecond2019,
	util.ProductionThird2019,
	util.ProductionFourth2019,
	// 2021 plans
	util.ProductionFirst2021,
	util.ProductionSecond2021,
	util.ProductionThird2021,
	util.HostedArcEnterprise2021,
	util.ProductionFirst2023,
}

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classify.Op(),
		classify.Indices(),
		logs.Recorder(),
		auth.BasicAuth(),
		validate.Sources(),
		validate.Indices(),
		validate.Operation(),
		validate.Category(),
		validate.Plan(validPlans, util.GetFeatureCache(), "Cache feature"),
		telemetry.Recorder(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		requestCategory := category.Cache

		ctx := category.NewContext(req.Context(), &requestCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

// Saves the request body to cache
func saveToCacheRSAPI(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		body, err := querytranslate.FromContext(req.Context())
		if err != nil {
			log.Errorln(logTag, ":", err)
			telemetry.WriteBackErrorWithTelemetry(req, w, "error encountered while retrieving request from context", http.StatusInternalServerError)
			return
		}
		if ShouldApplyCache(req, *body) {
			useCache := false
			if body.Settings != nil && body.Settings.UseCache != nil {
				useCache = *body.Settings.UseCache
			}
			// Track plugin
			ctx := trackplugin.TrackPlugin(req.Context(), "cr")
			r := req.WithContext(ctx)
			// Call record cache middleware
			recordCache(h, useCache)(w, r)
		} else {
			h(w, req)
		}
	}
}

func recordCache(h http.HandlerFunc, useCache bool) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Validate plan
		// Only throw 402 error when `useCache` is set in RS API request
		// otherwise ignore the cache middleware
		// the reason for not throwing error is to support a use case
		// when user downgrades their plan and caching was enabled
		if util.ValidatePlans(validPlans, util.GetFeatureCache()) {
			resp := httptest.NewRecorder()
			h.ServeHTTP(resp, req)
			// Copy the response to writer
			for k, v := range resp.Header() {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.Code)
			w.Write(resp.Body.Bytes())
			// Only cache success responses
			if resp.Code == http.StatusOK {
				requestBody, err := querytranslate.FromContext(req.Context())
				if err != nil {
					log.Errorln(logTag, ":", err)
					return
				}
				go RecordRequest(req.RequestURI, requestBody, nil, resp.Body.Bytes())
			}
		} else if useCache {
			msg := "Cache feature is not available for the free plan users, please upgrade to a paid plan or set `useCache` property to `false`."
			if util.GetTier() != nil {
				msg = "Cache feature is not available for the " + util.GetTier().String() + " plan users, please upgrade to a higher plan or set `useCache` property to `false`."
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusPaymentRequired)
		} else {
			// forward request
			h(w, req)
		}
	})
}

// This handler writes to request cache
func RecordRequest(urlPath string, rsAPIBody *querytranslate.RSQuery, reqBody []byte, response []byte) {
	out := reqBody
	if rsAPIBody != nil {
		sanitizedRequest := SanitizeRequest(*rsAPIBody)
		output, err := json.Marshal(sanitizedRequest)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return
		}
		out = output
	}
	if out != nil {
		writeToCache(urlPath, out, response, rsAPIBody)
	}
}
