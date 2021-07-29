package telemetry

import (
	"net"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
)

// Returns the server mode based on the billing type
func getServerMode() string {
	var serverMode string = defaultServerMode
	if util.ClusterBilling == "true" {
		serverMode = "Cloud"
	} else if util.HostedBilling == "true" {
		serverMode = "BYE"
	} else if util.Billing == "true" {
		serverMode = "Self-Host"
	}
	return serverMode
}

// Returns an ip address without last 8 bits (1 byte)
func getClientIP4(ip string) string {
	parsedIP := net.ParseIP(ip)
	// Remove last byte
	ipv4 := parsedIP.To4()
	if ipv4 != nil {
		splited := strings.Split(ipv4.String(), ".")
		splited[len(splited)-1] = "x"
		return strings.Join(splited, ".")
	}
	return ""
}

// Returns an ip address without last 8 bits (1 byte)
func getClientIP6(ip string) string {
	parsedIP := net.ParseIP(ip)
	// Remove last byte
	ipv6 := parsedIP.To16()
	ipv4 := parsedIP.To4()
	if ipv4 == nil && ipv6 != nil {
		splited := strings.Split(ipv6.String(), ":")
		splited[len(splited)-1] = "x"
		return strings.Join(splited, ":")
	}
	return ""
}
