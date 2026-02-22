package validate

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/middleware"
	"github.com/appbaseio/reactivesearch-api/plugins/telemetry"
	"github.com/appbaseio/reactivesearch-api/util"
)

// Plan returns a middleware that validates the user's plan.
// For e.g `validate.Plan([]util.Plan{util.ArcEnterprise}),` restricts the route to only appbase.io enterprise users.
func Plan(validPlans []util.Plan, byPassValidation bool, featureName string) middleware.Middleware {
	if util.ValidatePlans(validPlans, byPassValidation) {
		return validPlan
	}
	return invalidPlanMiddleware(featureName)
}

func invalidPlanMiddleware(featureName string) middleware.Middleware {
	return func(h http.HandlerFunc) http.HandlerFunc {
		feature := featureName
		if feature == "" {
			feature = "This feature"
		}
		return func(w http.ResponseWriter, req *http.Request) {
			msg := feature + " is not available for the free plan users, please upgrade to a paid plan."
			if util.GetTier() != nil {
				msg = feature + " is not available for the " + util.GetTier().String() + " plan users, please upgrade to a higher plan."
			}
			telemetry.WriteBackErrorWithTelemetry(req, w, msg, http.StatusPaymentRequired)
		}
	}
}

// Authorize to access the request
func validPlan(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		h(w, req)
	}
}
