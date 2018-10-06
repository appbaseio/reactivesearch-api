package iplookup

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
)

const ipLookupURL = "http://extreme-ip-lookup.com/json/"

type Info int

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
	instance *IpInfo
	once     sync.Once
)

type IpInfo struct {
	sync.Mutex
	cache map[string]*IpLookup
}

type IpLookup struct {
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

func Instance() *IpInfo {
	once.Do(func() {
		instance = &IpInfo{cache: make(map[string]*IpLookup)}
	})
	return instance
}

func (info *IpInfo) Cached(ipAddr string) (*IpLookup, bool) {
	info.Lock()
	defer info.Unlock()
	if ip, ok := info.cache[ipAddr]; ok {
		return ip, true
	}
	return nil, false
}

func (info *IpInfo) Cache(ip string, ipLookup *IpLookup) {
	info.Lock()
	defer info.Unlock()
	info.cache[ip] = ipLookup
}

func (info *IpInfo) Lookup(ip string) (*IpLookup, error) {
	if ip, ok := info.Cached(ip); ok {
		return ip, nil
	}

	response, err := http.Get(ipLookupURL + ip)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var ipLookup IpLookup
	if err := json.Unmarshal(responseBody, &ipLookup); err != nil {
		return nil, err
	}

	info.Cache(ip, &ipLookup)
	return &ipLookup, nil
}

func (info *IpInfo) Get(field Info, ip string) (string, error) {
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
		return "", errors.New(fmt.Sprintf("cannot fetch %v from %s", field, ipLookupURL))
	}

	return ipInfo, nil
}

func (info *IpInfo) GetCoordinates(ip string) (string, error) {
	ipLookup, err := info.Lookup(ip)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s, %s", ipLookup.Lat, ipLookup.Lon), nil
}
