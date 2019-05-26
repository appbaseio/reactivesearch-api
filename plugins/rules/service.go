package rules

import (
	"context"

	"github.com/appbaseio-confidential/arc/plugins/rules/query"
)

type rulesService interface {
	postIndexRule(ctx context.Context, indexName string, rule *query.Rule) (bool, error)
	postIndexRules(ctx context.Context, indexName string, rules []query.Rule) (bool, error)
	getIndexRules(ctx context.Context, indexName string) ([]byte, error)
	getIndexRuleWithID(ctx context.Context, indexName, ruleID string) ([]byte, error)
	fetchQueryRules(ctx context.Context, indexName, query string, rules chan<- *query.Rule)
	deleteIndexRules(ctx context.Context, indexName string) (bool, error)
	deleteIndexRuleWithID(ctx context.Context, indexName, ruleID string) (bool, error)
}
