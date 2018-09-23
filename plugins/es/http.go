package es

import (
	"net"
	"net/http"
	"time"
)

var client *http.Client

// See: https://medium.com/@nate510/don-t-use-go-s-default-http-client-4804cb19f779
// This client will cap the TCP connect and TLS handshake timeouts,
// as well as establishing an end-to-end request timeout.
func httpClient() *http.Client {
	if client == nil {
		var netTransport = &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 5 * time.Second,
		}
		var netClient = &http.Client{
			Timeout:   time.Second * 10,
			Transport: netTransport,
		}
		client = netClient
	}
	return client
}
