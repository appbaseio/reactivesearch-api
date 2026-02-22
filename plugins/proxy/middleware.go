package proxy

import (
	"github.com/appbaseio/reactivesearch-api/util"
)

type getArcDetails struct {
	Message      string                    `json:"message"`
	ArcInstances []util.ArcInstanceDetails `json:"instances"`
}

type deleteArcSubscription struct {
	OTP string `json:"otp"`
}

type ClusterDetails struct {
	Plan util.ClusterPlan `json:"plan"`
}
