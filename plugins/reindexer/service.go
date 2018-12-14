package reindexer

type reindexService interface {
	reindex(index string, mappings, settings map[string]interface{}, includes, excludes, types []string) error
	mappingsOf(index string) (map[string]interface{}, error)
	settingsOf(index string) (map[string]interface{}, error)
	aliasesOf(index string) ([]string, error)
	createIndex(name string, body map[string]interface{}) error
	deleteIndex(name string) error
	setAlias(index string, aliases ...string) error
	getIndicesByAlias(alias string) ([]string, error)
}