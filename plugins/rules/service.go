package rules

import (
	"context"
	"sync"

	"github.com/appbaseio-confidential/arc/plugins/rules/query"
)

type rulesService interface {
	postRule(ctx context.Context, indexName string, rule query.Rule) (bool, error)
	getIndexRules(ctx context.Context, indexName string) ([]byte, error)
	fetchQueryRules(ctx context.Context, indexName, query string, rules chan<- query.Rule)
	fetchDoc(ctx context.Context, indexName, docID string, docs chan<- *indexDoc, wg *sync.WaitGroup)
}
