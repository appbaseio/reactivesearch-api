package util

// DomainToTenant will contain the tenantID
// mapped to the domain of the tenant
type DomainToTenant map[string]string

var domainMap DomainToTenant

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
