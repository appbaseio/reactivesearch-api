package iplookup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/appbaseio/reactivesearch-api/util"
)

const ipLookupURL = "https://extreme-ip-lookup.com/json/"

// Info is the information associated with an IP address provided by ip-lookup service.
type Info int

// Information fetched from an IP address.
const (
	BusinessName Info = iota
	BusinessWebsite
	City
	Continent
	Country
	CountryCode
	IPName
	IPType
	ISP
	Lat
	Lon
	Org
	Query
	Region
	Status
)

var (
	instance *IPInfo
	once     sync.Once
)

// IPInfo maintains a cache to hold IpLookup information to avoid redundant
// network requests made for the same IP address.
type IPInfo struct {
	sync.Mutex
	cache map[string]*IPLookup
}

// IPLookup represents the response received from the ip-llokup service.
type IPLookup struct {
	BusinessName    string `json:"businessName"`
	BusinessWebsite string `json:"businessWebsite"`
	City            string `json:"city"`
	Continent       string `json:"continent"`
	Country         string `json:"country"`
	CountryCode     string `json:"countryCode"`
	IPName          string `json:"ipName"`
	IPType          string `json:"ipType"`
	ISP             string `json:"isp"`
	Lat             string `json:"lat"`
	Lon             string `json:"lon"`
	Org             string `json:"org"`
	Query           string `json:"query"`
	Region          string `json:"region"`
	Status          string `json:"status"`
}

// Instance returns the singleton instance of IPInfo.
func Instance() *IPInfo {
	once.Do(func() {
		instance = &IPInfo{cache: make(map[string]*IPLookup)}
	})
	return instance
}

// Cached checks if the info for the ipAddr is present in the cache. If so
// we return the result from the cache itself.
func (info *IPInfo) Cached(ipAddr string) (*IPLookup, bool) {
	info.Lock()
	defer info.Unlock()
	if ip, ok := info.cache[ipAddr]; ok {
		return ip, true
	}
	return nil, false
}

// Cache stores the IP information i.e. IPLookup in the cache.
func (info *IPInfo) Cache(ip string, ipLookup *IPLookup) {
	info.Lock()
	defer info.Unlock()
	info.cache[ip] = ipLookup
}

// Lookup fetches the ip information from the ip-lookup service. A request to
// ip-lookup service is made only when the information is not available in the cache.
func (info *IPInfo) Lookup(ip string) (*IPLookup, error) {
	if ip, ok := info.Cached(ip); ok {
		return ip, nil
	}

	key := "demo"
	url := ipLookupURL + ip + "?key=" + key
	if util.IsBillingEnabled() && !util.OfflineBilling {
		clusterID, err := util.GetArcID()
		if err != nil {
			return nil, err
		}
		url = util.ACCAPI + "arc/iplookup/" + clusterID + "/" + ip
	}
	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var ipLookup IPLookup
	if err := json.Unmarshal(responseBody, &ipLookup); err != nil {
		return nil, err
	}

	info.Cache(ip, &ipLookup)
	return &ipLookup, nil
}

// Get returns the specific field of information i.e. Info from IPLookup.
func (info *IPInfo) Get(field Info, ip string) (string, error) {
	ipLookup, err := info.Lookup(ip)
	if err != nil {
		return "", err
	}
	var ipInfo string
	switch field {
	case BusinessName:
		ipInfo = ipLookup.BusinessName
	case BusinessWebsite:
		ipInfo = ipLookup.BusinessWebsite
	case City:
		ipInfo = ipLookup.City
	case Continent:
		ipInfo = ipLookup.Continent
	case Country:
		ipInfo = ipLookup.Country
	case CountryCode:
		ipInfo = ipLookup.CountryCode
	case IPName:
		ipInfo = ipLookup.IPName
	case IPType:
		ipInfo = ipLookup.IPType
	case ISP:
		ipInfo = ipLookup.ISP
	case Lat:
		ipInfo = ipLookup.Lat
	case Lon:
		ipInfo = ipLookup.Lon
	case Org:
		ipInfo = ipLookup.Org
	case Query:
		ipInfo = ipLookup.Query
	case Region:
		ipInfo = ipLookup.Region
	case Status:
		ipInfo = ipLookup.Status
	default:
		return "", fmt.Errorf("cannot fetch %v from %s", field, ipLookupURL)
	}

	return ipInfo, nil
}

// GetCoordinates returns the formatted coordinates (both latitude and longitude)
// of the location fetched for IP.
func (info *IPInfo) GetCoordinates(ip string) (string, error) {
	ipLookup, err := info.Lookup(ip)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s, %s", ipLookup.Lat, ipLookup.Lon), nil
}
