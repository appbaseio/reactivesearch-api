package elasticsearch

import "sort"

// Store the indices for tenants using a tenant to index map
//
// The name of the index will be stored without the `tenant_id`
// appended in it.
var tenantToIndexMap map[string][]string = make(map[string][]string)

// SetIndexToCache will set the index into the cache map
func SetIndexToCache(tenantID string, index string) {
	// Check if the entry for the tenantID already exists,
	// if it doesn't exist, then create it.
	_, keyExists := tenantToIndexMap[tenantID]
	if !keyExists {
		tenantToIndexMap[tenantID] = make([]string, 0)
	}

	tenantToIndexMap[tenantID] = append(tenantToIndexMap[tenantID], index)
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

	return true
}

// GetCachedIndices will return the cached indices so that all indices
// can be iterated over in an O(n) instead of O(n2).
//
// The indexes will be sorted and returned in terms of length of
// the index name where the longer index name should show up first.
func GetCachedIndices(tenantID string) []string {
	cachedIndices, exists := tenantToIndexMap[tenantID]
	if !exists {
		return make([]string, 0)
	}

	sort.Slice(cachedIndices, func(i, j int) bool {
		return len(cachedIndices[i]) > len(cachedIndices[j])
	})
	return cachedIndices
}
