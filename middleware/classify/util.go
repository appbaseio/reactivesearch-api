package classify

// IndexAliasCache cache to stor index alias map
var IndexAliasCache = map[string]string{}

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
