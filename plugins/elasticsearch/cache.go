package elasticsearch

// Store the indices for tenants using a tenant to index map
//
// The name of the index will be stored without the `tenant_id`
// appended in it.
var tenantToIndexMap map[string][]string

// SetIndexesToCache will set the index into the cache map
func SetIndexesToCache(tenantID string, index string) {
	// Check if the entry for the tenantID already exists,
	// if it doesn't exist, then create it.
	_, keyExists := tenantToIndexMap[tenantID]
	if !keyExists {
		tenantToIndexMap[tenantID] = make([]string, 0)
	}

	tenantToIndexMap[tenantID] = append(tenantToIndexMap[tenantID], index)
}
