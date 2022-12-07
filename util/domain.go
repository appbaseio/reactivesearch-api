package util

import (
	log "github.com/sirupsen/logrus"
)

// DomainToTenant will contain the tenantID
// mapped to the domain of the tenant
type DomainToTenant map[string]string

var domainMap DomainToTenant

// GetDomainMap will return the domain map
func GetDomainMap() *DomainToTenant {
	return &domainMap
}

// GetTenantForDomain will return the tenantID for the domain
// passed.
func (dt *DomainToTenant) GetTenantForDomain(domain string) string {
	tenantId, exists := (*dt)[domain]
	if !exists {
		return ""
	}

	return tenantId
}

// FetchDomainMap will fetch the domain to tenant map
// from AccAPI and return it
func FetchDomainMap() (map[string]string, error) {
	// TODO: Add code to fetch the map and return it
	return make(map[string]string), nil
}

// SetDomainInCache will fetch the domain map from AccAPI
// and set it in the local cache of the DomainToTenant
func SetDomainInCache() error {
	domainMapFetched, fetchErr := FetchDomainMap()
	if fetchErr != nil {
		return fetchErr
	}

	domainMap = domainMapFetched
	return nil
}

// SetDomainInCacheCronFunc is a wrapper on top of
// SetDomainInCache to handle errors and report them
// gracefully.
func SetDomainInCacheCronFunc() {
	setErr := SetDomainInCache()
	if setErr != nil {
		log.Warnln("Error while updating domain map cache: ", setErr.Error())
	}
}
