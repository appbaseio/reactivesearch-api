package rules

import (
	"context"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(rulesIndex, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{rulesIndex}

	ctx := context.Background()

	// Check if the rules index already exists
	exists, err := util.GetClient7().IndexExists(rulesIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, rulesIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, rulesMapping, util.HiddenIndexSettings(), replicas)

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(rulesIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", rulesIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, rulesIndex)
	return es, nil
}

// To create rule in ES index
func (es *elasticsearch) createRule(ctx context.Context, ruleID string, record ESRuleDoc) error {
	return es.createRuleEs7(ctx, ruleID, record)
}

// To update a rule in ES index
func (es *elasticsearch) updateRule(ctx context.Context, ruleID string, record ESRuleDoc) error {
	return es.updateRuleEs7(ctx, ruleID, record)
}

// To delte a rule in ES index
func (es *elasticsearch) deleteRule(ctx context.Context, ruleID string) error {
	return es.deleteRuleEs7(ctx, ruleID)
}

// To get rules from ES index
func (es *elasticsearch) getRules(ctx context.Context) ([]ESRuleDoc, error) {
	return es.getRulesEs7(ctx)
}

// To get rule from ES index
func (es *elasticsearch) getRule(ctx context.Context, ruleId string) (*ESRuleDoc, error) {
	return es.getRuleEs7(ctx, ruleId)
}

// To get the rules size
func (es *elasticsearch) getRulesSize(ctx context.Context) (*int64, error) {
	return es.getRulesSizeEs7(ctx)
}

// To validate the query string
func (es *elasticsearch) validateQuery(ctx context.Context, query string) (bool, error) {
	return es.validateQueryEs7(ctx, query)
}
