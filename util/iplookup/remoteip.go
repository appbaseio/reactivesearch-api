package iplookup

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/appbaseio/reactivesearch-api/model/credential"
	"github.com/appbaseio/reactivesearch-api/model/permission"
	"github.com/appbaseio/reactivesearch-api/model/user"
)

var cidrs []*net.IPNet

func init() {
	maxCIDRBlocks := []string{
		"127.0.0.1/8",    // localhost
		"10.0.0.0/8",     // 24-bit block
		"172.16.0.0/12",  // 20-bit block
		"192.168.0.0/16", // 16-bit block
		"169.254.0.0/16", // link local address
		"::1/128",        // localhost IPv6
		"fc00::/7",       // unique local address IPv6
		"fe80::/10",      // link local address IPv6
	}

	cidrs = make([]*net.IPNet, len(maxCIDRBlocks))
	for i, maxCIDRBlock := range maxCIDRBlocks {
		_, cidr, _ := net.ParseCIDR(maxCIDRBlock)
		cidrs[i] = cidr
	}
}

// isLocalAddress works by checking if the address is under private CIDR blocks.
// List of private CIDR blocks can be seen on :
//
// https://en.wikipedia.org/wiki/Private_network
// https://en.wikipedia.org/wiki/Link-local_address
func isPrivateAddress(address string) (bool, error) {
	ipAddress := net.ParseIP(address)
	if ipAddress == nil {
		return false, errors.New("address is not valid")
	}

	for i := range cidrs {
		if cidrs[i].Contains(ipAddress) {
			return true, nil
		}
	}

	return false, nil
}

// FromRequest identifies the remote ip from an http request
func FromRequest(r *http.Request) string {
	// Fetch header value
	xRealIP := r.Header.Get("X-Real-Ip")
	xForwardedFor := r.Header.Get("X-Forwarded-For")
	sourcesXffValue := 0
	reqCredential, _ := credential.FromContext(r.Context())
	if reqCredential == credential.User {
		reqUser, err := user.FromContext(r.Context())
		if err == nil && reqUser != nil && reqUser.SourcesXffValue != nil {
			sourcesXffValue = *reqUser.SourcesXffValue
		}
	} else {
		reqPermission, err := permission.FromContext(r.Context())
		if err == nil && reqPermission != nil && reqPermission.SourcesXffValue != nil {
			sourcesXffValue = *reqPermission.SourcesXffValue
		}
	}
	if xForwardedFor != "" {
		// Check list of IP in X-Forwarded-For and return the first global address
		ipAddresses := strings.Split(xForwardedFor, ",")
		if sourcesXffValue != 0 {
			// if xffSourceValue is invalid then throw error
			if sourcesXffValue < len(ipAddresses) {
				address := strings.TrimSpace(ipAddresses[sourcesXffValue-1])
				isPrivate, err := isPrivateAddress(address)
				if !isPrivate && err == nil {
					return address
				}
			}
		} else {
			for _, address := range ipAddresses {
				address = strings.TrimSpace(address)
				isPrivate, err := isPrivateAddress(address)
				if !isPrivate && err == nil {
					return address
				}
			}
		}
	}

	// If xRealIP empty, return IP from remote address
	if xRealIP == "" {
		var remoteIP string

		// If there are colon in remote address, remove the port number
		// otherwise, return remote address as is
		if strings.ContainsRune(r.RemoteAddr, ':') {
			remoteIP, _, _ = net.SplitHostPort(r.RemoteAddr)
		} else {
			remoteIP = r.RemoteAddr
		}

		return remoteIP
	}

	// If nothing succeed, return X-Real-IP
	return xRealIP
}
