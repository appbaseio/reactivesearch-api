package iplookup

import (
	"errors"
	"net"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
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
	log.Errorln("REMOTE IP: ", "xRealIP =>", xRealIP, "xForwardedFor =>", xForwardedFor)
	log.Errorln("REMOTE IP: ", "r.RemoteAddr =>", r.RemoteAddr)
	// If both empty, return IP from remote address
	if xRealIP == "" && xForwardedFor == "" {
		var remoteIP string

		// If there are colon in remote address, remove the port number
		// otherwise, return remote address as is
		if strings.ContainsRune(r.RemoteAddr, ':') {
			remoteIP, _, _ = net.SplitHostPort(r.RemoteAddr)
		} else {
			remoteIP = r.RemoteAddr
		}
		log.Errorln("REMOTE IP: ", "remoteIP =>", remoteIP)
		return remoteIP
	}

	// Check list of IP in X-Forwarded-For and return the first global address
	for _, address := range strings.Split(xForwardedFor, ",") {
		address = strings.TrimSpace(address)
		isPrivate, err := isPrivateAddress(address)
		if !isPrivate && err == nil {
			log.Errorln("REMOTE IP: ", "returning address =>", address)
			return address
		}
	}
	log.Errorln("REMOTE IP: ", "returning xRealIP =>", xRealIP)
	// If nothing succeed, return X-Real-IP
	return xRealIP
}
