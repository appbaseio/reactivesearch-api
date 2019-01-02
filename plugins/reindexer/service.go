package reindexer

import "context"

type reindexService interface {
	reindex(ctx context.Context, index string, mappings, settings map[string]interface{}, includes, excludes, types []string) error
	mappingsOf(ctx context.Context, index string) (map[string]interface{}, error)
	settingsOf(ctx context.Context, index string) (map[string]interface{}, error)
	aliasesOf(ctx context.Context, index string) ([]string, error)
	createIndex(ctx context.Context, name string, body map[string]interface{}) error
	deleteIndex(ctx context.Context, name string) error
	setAlias(ctx context.Context, index string, aliases ...string) error
	getIndicesByAlias(ctx context.Context, alias string) ([]string, error)
}
