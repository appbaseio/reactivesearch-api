package rules

import (
	"context"
	"encoding/json"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) createRuleEs7(ctx context.Context, ruleID string, record ESRuleDoc) error {
	_, err := util.GetClient7().
		Index().
		Index(es.indexName).
		BodyJson(record).
		Refresh("wait_for").
		Id(ruleID).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error indexing rule record:", err)
		return err
	}
	return nil
}

func (es *elasticsearch) updateRuleEs7(ctx context.Context, ruleID string, record ESRuleDoc) error {
	_, err := util.GetClient7().
		Update().
		Index(es.indexName).
		Id(ruleID).
		Doc(record).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error updating rule record:", err)
		return err
	}
	return nil
}

func (es *elasticsearch) deleteRuleEs7(ctx context.Context, ruleID string) error {
	_, err := util.GetClient7().
		Delete().
		Index(es.indexName).
		Id(ruleID).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error deleteing rule record:", err)
		return err
	}
	return nil
}

func (es *elasticsearch) getRulesEs7(ctx context.Context) ([]ESRuleDoc, error) {
	response, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Size(int(getSizeLimitByPlan())).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving rules record:", err)
		return nil, err
	}

	var final []ESRuleDoc

	// NOTE: The following should not happen anymore with the es7 client either.
	// On version 8, then the response object will not have a valid Hits value
	// so it will end up with an out of memory error.
	if response.Hits == nil || response.Hits.Hits == nil {
		return final, nil
	}

	for _, hit := range response.Hits.Hits {
		var rule ESRuleDoc
		err := json.Unmarshal(hit.Source, &rule)
		if err != nil {
			log.Errorln(logTag, ": error while unmarshalling rule record:", err)
		} else {
			final = append(final, rule)
		}
	}

	return final, nil
}

func (es *elasticsearch) getRulesSizeEs7(ctx context.Context) (*int64, error) {
	response, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Size(0).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving rules size:", err)
		return nil, err
	}
	return &response.Hits.TotalHits.Value, nil
}

func (es *elasticsearch) validateQueryEs7(ctx context.Context, query string) (bool, error) {
	// Just use .rules index for validation
	response, err := util.GetClient7().Validate(es.indexName).Query(es7.NewQueryStringQuery(query)).Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error while validating query string:", err)
		return false, err
	}
	return response.Valid, nil
}

func (es *elasticsearch) getRuleEs7(ctx context.Context, ruleId string) (*ESRuleDoc, error) {
	response, err := util.GetClient7().
		Get().
		Id(ruleId).
		Index(es.indexName).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving rules record:", err)
		return nil, err
	}

	var final ESRuleDoc

	var rule ESRuleDoc
	err2 := json.Unmarshal(response.Source, &rule)
	if err2 != nil {
		log.Errorln(logTag, ": error while unmarshalling rule record:", err)
	} else {
		final = rule
	}

	return &final, nil
}
