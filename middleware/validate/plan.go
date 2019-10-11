package validate

import (
	"net/http"

	"github.com/appbaseio/arc/middleware"
	"github.com/appbaseio/arc/util"
)

// Plan returns a middleware that validates the user's plan.
// For e.g `validate.Plan([]util.Plan{util.ArcBasic}),` restricts the route to arc-basic users.
func Plan(restrictedPlans []util.Plan) middleware.Middleware {
	if util.ValidatedPlans(restrictedPlans) {
		return validPlan
	}
	return invalidPlan
}

// Throws the payment required error
func invalidPlan(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		msg := "This feature is not available for the " + util.Tier.String() + " plan users."
		util.WriteBackError(w, msg, http.StatusPaymentRequired)
	}
}

// Authorize to access the request
func validPlan(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		h(w, req)
	}
}
