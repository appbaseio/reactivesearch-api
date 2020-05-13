package classify

// IndexAliasCache cache to store index -> alias map
var IndexAliasCache = make(map[string]string)

// AliasIndexCache cache to store alias -> index map
var AliasIndexCache = make(map[string]string)

// GetIndexAliasCache get whole cache
func GetIndexAliasCache() map[string]string {
	return IndexAliasCache
}

// GetIndexAlias get alias for specific index
func GetIndexAlias(index string) string {
	alias, ok := IndexAliasCache[index]

	if !ok {
		return ""
	}
	return alias
}

// SetIndexAlias set alias for specific index
func SetIndexAlias(index, alias string) {
	IndexAliasCache[index] = alias
}

// GetAliasIndex get index for specific alias
func GetAliasIndex(alias string) string {
	index, ok := AliasIndexCache[alias]
	if !ok {
		return ""
	}
	return index
}

// SetAliasIndex set index for specific alias
func SetAliasIndex(alias, index string) {
	AliasIndexCache[alias] = index
}

// SetAliasIndexCache set the whole cache
func SetAliasIndexCache(data map[string]string) {
	AliasIndexCache = data
}

// GetAliasIndexCache get the whole cache
func GetAliasIndexCache() map[string]string {
	return AliasIndexCache
}

// RemoveFromIndexAliasCache get the whole cache
func RemoveFromIndexAliasCache(indexName string) {
	delete(IndexAliasCache, indexName)
}
