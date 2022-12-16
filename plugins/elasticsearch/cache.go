package elasticsearch

// Store the indices for tenants using a tenant to index map
//
// The name of the index will be stored without the `tenant_id`
// appended in it.
var tenantToIndexMap map[string][]string = make(map[string][]string)

// Store the list of indices without tenant ID map. This can be useful
// for updating incoming requests like _msearch and _bulk with the index
// names
//
// We will use a set to store the values to have unique values only.
var cachedIndices = make(map[string]int)

// SetIndexToCache will set the index into the cache map
func SetIndexToCache(tenantID string, index string) {
	// Check if the entry for the tenantID already exists,
	// if it doesn't exist, then create it.
	_, keyExists := tenantToIndexMap[tenantID]
	if !keyExists {
		tenantToIndexMap[tenantID] = make([]string, 0)
	}

	tenantToIndexMap[tenantID] = append(tenantToIndexMap[tenantID], index)

	SetCachedIndex(index)
}

// GetIndexLocFromCache will try to get the index by using
// the passed tenantId and the index name
func GetIndexLocFromCache(tenantID string, index string) *int {
	indicesList, exists := tenantToIndexMap[tenantID]
	if !exists {
		return nil
	}

	for indexPosition, indexName := range indicesList {
		if indexName == index {
			return &indexPosition
		}
	}

	return nil
}

// DeleteIndexFromCache will remove the index from the cache
// map
func DeleteIndexFromCache(tenantID string, index string) bool {
	location := GetIndexLocFromCache(tenantID, index)
	if location == nil {
		return false
	}

	tenantToIndexMap[tenantID] = append(tenantToIndexMap[tenantID][:*location], tenantToIndexMap[tenantID][*location+1:]...)

	RemoveCachedIndex(index)

	return true
}

// SetCachedIndex will allow setting a cached index
func SetCachedIndex(index string) {
	cachedIndices[index] = 1
}

// RemoveCachedIndex will remove the index from the cached array of indices
func RemoveCachedIndex(index string) {
	delete(cachedIndices, index)
}

// GetCachedIndices will return the cached indices so that all indices
// can be iterated over in an O(n) instead of O(n2).
func GetCachedIndices() []string {
	indices := make([]string, 0)
	for index := range cachedIndices {
		indices = append(indices, index)
	}

	return indices
}
