package validate

import (
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/util"
)

// Plan returns a middleware that validates the user's plan.
// For e.g `validate.Plan([]util.Plan{util.ArcEnterprise}),` restricts the route to only arc-enterprise users.
func Plan(validPlans []util.Plan, byPassValidation bool) middleware.Middleware {
	if util.ValidatePlans(validPlans, byPassValidation) {
		return validPlan
	}
	return invalidPlan
}

// Throws the payment required error
func invalidPlan(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		msg := "This feature is not available for the free plan users, please upgrade to a paid plan."
		if util.Tier != nil {
			msg = "This feature is not available for the " + util.Tier.String() + " plan users, please upgrade to a higher plan."
		}
		util.WriteBackError(w, msg, http.StatusPaymentRequired)
	}
}

// Authorize to access the request
func validPlan(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		h(w, req)
	}
}
