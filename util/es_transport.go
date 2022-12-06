package util

import (
	"net/http"

	log "github.com/sirupsen/logrus"
)

// CustomESTransport will be passed to olivere/elasticsearch
type CustomESTransport struct {
	originalTransport http.RoundTripper
}

// RoundTrip will add a header to every ES request.
func (ct *CustomESTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Only add `X-Appbase-ID` for multi-tenant ARC
	if !MultiTenant {
		arcId, arcIdErr := GetArcID()
		if arcIdErr != nil {
			log.Errorln("error while getting arc ID to add it to every ES request, ", arcIdErr)
			return http.DefaultTransport.RoundTrip(req)
		}
		req.Header.Add("X-Appbase-ID", arcId)
	}
	return ct.originalTransport.RoundTrip(req)
}
