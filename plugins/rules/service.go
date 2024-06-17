package rules

import "context"

type rulesService interface {
	createRule(ctx context.Context, ruleID string, record ESRuleDoc) error
	updateRule(ctx context.Context, ruleID string, record ESRuleDoc) error
	deleteRule(ctx context.Context, ruleID string) error
	getRules(ctx context.Context) ([]ESRuleDoc, error)
	getRule(ctx context.Context, ruleId string) (*ESRuleDoc, error)
	getRulesSize(ctx context.Context) (*int64, error)
	validateQuery(ctx context.Context, query string) (bool, error)
}
