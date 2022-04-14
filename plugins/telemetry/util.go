package telemetry

import (
	"net"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
	badger "github.com/outcaste-io/badger/v3"
)

// Returns the server mode based on the billing type
func getServerMode() string {
	serverMode := getCustomer()
	if serverMode == "" {
		serverMode = defaultServerMode
	}
	return serverMode
}

// Returns the type of the customer
func getCustomer() string {
	var serverMode string
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

func setBadgerOptions(filePath string) badger.Options {
	opts := badger.DefaultOptions(filePath)
	opts.NumMemtables = 2

	// The NumLevelZeroTables and NumLevelZeroTableStall will not have any
	// effect on the memory if `KeepL0InMemory` is set to false.
	opts.NumLevelZeroTables = 1
	opts.NumLevelZeroTablesStall = 2

	// SyncWrites=false has significant effect on write performance. When sync
	// writes is set to true, badger ensures the data is flushed to the disk after a
	// write call. For normal usage, such a high level of consistency is not required.
	opts.SyncWrites = false

	return opts
}
