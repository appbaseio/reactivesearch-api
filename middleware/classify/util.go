package classify

// IndexAliasCache cache to store tenantId -> index -> alias map
var IndexAliasCache = make(map[string]map[string]string)

// AliasIndexCache cache to store tenantId -> alias -> index map
var AliasIndexCache = make(map[string]map[string]string)

// GetIndexAliasCache get whole cache
func GetIndexAliasCache() map[string]map[string]string {
	return IndexAliasCache
}

// GetIndexAlias get alias for specific index
func GetIndexAlias(tenantId, index string) string {
	if domainMap, ok := IndexAliasCache[index]; ok {
		if alias, ok := domainMap[index]; ok {
			return alias
		}
	}
	return ""
}

// SetIndexAlias set alias for specific index
func SetIndexAlias(tenantId, index, alias string) {
	if _, ok := IndexAliasCache[index]; ok {
		IndexAliasCache[tenantId][index] = alias
	} else {
		IndexAliasCache[tenantId] = map[string]string{
			index: alias,
		}
	}
}

// GetAliasIndex get index for specific alias
func GetAliasIndex(tenantId string, alias string) string {
	if domainMap, ok := IndexAliasCache[alias]; ok {
		if index, ok := domainMap[alias]; ok {
			return index
		}
	}
	return ""
}

// SetAliasIndex set index for specific alias
func SetAliasIndex(tenantId, alias, index string) {
	if _, ok := IndexAliasCache[alias]; ok {
		IndexAliasCache[tenantId][alias] = index
	} else {
		IndexAliasCache[tenantId] = map[string]string{
			alias: index,
		}
	}
}

// SetAliasIndexCache set the whole cache
func SetAliasIndexCache(data map[string]map[string]string) {
	AliasIndexCache = data
}

// GetAliasIndexCache get the whole cache
func GetAliasIndexCache() map[string]map[string]string {
	return AliasIndexCache
}

// RemoveFromIndexAliasCache get the whole cache
func RemoveFromIndexAliasCache(tenantId, indexName string) {
	delete(IndexAliasCache, indexName)
}
