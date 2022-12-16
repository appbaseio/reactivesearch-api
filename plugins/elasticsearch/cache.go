package elasticsearch

import "sort"

// Store the indices for tenants using a tenant to index map
//
// The name of the index will be stored without the `tenant_id`
// appended in it.
var tenantToIndexMap map[string]map[string]int = make(map[string]map[string]int)

// SetIndexToCache will set the index into the cache map
func SetIndexToCache(tenantID string, index string) {
	// Check if the entry for the tenantID already exists,
	// if it doesn't exist, then create it.
	_, keyExists := tenantToIndexMap[tenantID]
	if !keyExists {
		tenantToIndexMap[tenantID] = make(map[string]int)
	}

	tenantToIndexMap[tenantID][index] = 1
}

// DeleteIndexFromCache will remove the index from the cache
// map
func DeleteIndexFromCache(tenantID string, index string) bool {
	delete(tenantToIndexMap[tenantID], index)
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

	indicesArr := make([]string, 0)
	for cachedIndex := range cachedIndices {
		indicesArr = append(indicesArr, cachedIndex)
	}

	sort.Slice(indicesArr, func(i, j int) bool {
		return len(indicesArr[i]) > len(indicesArr[j])
	})
	return indicesArr
}
