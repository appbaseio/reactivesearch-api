package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/arc/plugins/rules/query"
	"github.com/olivere/elastic"
)

type elasticsearch struct {
	url         string
	indexSuffix string
	indexConfig string
	typeName    string
	client      *elastic.Client
}

func newClient(url, indexSuffix, mapping string) (rulesService, error) {
	opts := []elastic.ClientOptionFunc{
		elastic.SetURL(url),
		elastic.SetSniff(false),
	}

	client, err := elastic.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("%s: error while initializing elastic client: %v", logTag, err)
	}
	es := &elasticsearch{
		url:         url,
		indexSuffix: indexSuffix,
		typeName:    "_doc",
		client:      client,
	}

	nodes, err := es.getTotalNodes()
	if err != nil {
		return nil, err
	}
	es.indexConfig = fmt.Sprintf(mapping, nodes, nodes-1)

	return es, nil
}

func (es *elasticsearch) getTotalNodes() (int, error) {
	response, err := es.client.NodesInfo().
		Metric("nodes").
		Do(context.Background())
	if err != nil {
		return -1, err
	}

	return len(response.Nodes), nil
}

func (es *elasticsearch) createIndex(ctx context.Context, indexName string) (bool, error) {
	response, err := es.client.CreateIndex(indexName).
		Body(es.indexConfig).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return response.Acknowledged, nil
}

func (es *elasticsearch) postIndexRule(ctx context.Context, indexName string, rule *query.Rule) (bool, error) {
	// e.g: app-rules
	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	// check if index already exists, this is important
	// because we need to set the percolator mapping to the
	// index before indexing the rules.
	exists, err := es.indexExists(ctx, indexName)
	if err != nil {
		return false, err
	}
	if !exists {
		if created, err := es.createIndex(ctx, indexName); err != nil || !created {
			return false, err
		}
	}

	_, err = es.client.Index().
		Index(indexName).
		Type(es.typeName).
		Id(rule.ID).
		BodyJson(*rule).
		Do(ctx)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (es *elasticsearch) postIndexRules(ctx context.Context, indexName string, rules []query.Rule) (bool, error) {
	// e.g: app-rules
	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	// check if index already exists, this is important
	// because we need to set the percolator mapping to the
	// index before indexing the rules.
	exists, err := es.indexExists(ctx, indexName)
	if err != nil {
		return false, err
	}
	if !exists {
		if created, err := es.createIndex(ctx, indexName); err != nil || !created {
			return false, err
		}
	}

	bulkRequest := es.client.Bulk()
	for _, rule := range rules {
		br := elastic.NewBulkIndexRequest().
			Index(indexName).
			Type(es.typeName).
			Id(rule.ID).
			Doc(rule)
		bulkRequest.Add(br)
	}
	_, err = bulkRequest.Do(ctx)
	if err != nil {
		return false, nil
	}

	return true, nil
}

func (es *elasticsearch) getIndexRules(ctx context.Context, indexName string) ([]byte, error) {
	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	response, err := es.client.Search().
		Index(indexName).
		Type(es.typeName).
		FetchSourceContext(elastic.NewFetchSourceContext(true).
			Exclude("query")).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	var raw []*json.RawMessage
	for _, hit := range response.Hits.Hits {
		raw = append(raw, hit.Source)
	}

	return json.Marshal(raw)
}

func (es *elasticsearch) fetchQueryRules(ctx context.Context, indexName, queryTerm string, rules chan<- *query.Rule) {
	defer close(rules)
	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	exists, err := es.indexExists(ctx, indexName)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		return
	}
	// rules index doesn't exist
	if !exists {
		return
	}

	doc := map[string]interface{}{"if.query": queryTerm}
	pq := elastic.NewPercolatorQuery().
		Field("query").
		Document(doc)

	excludeQuery := elastic.NewFetchSourceContext(true).
		Exclude("query")

	response, err := es.client.Search(indexName).
		FetchSourceContext(excludeQuery).
		Type(es.typeName).
		Query(pq).
		Do(ctx)
	if err != nil {
		log.Printf("%s: %v", logTag, err)
		return
	}

	for _, hit := range response.Hits.Hits {
		var rule query.Rule
		err := json.Unmarshal(*hit.Source, &rule)
		if err != nil {
			log.Printf("%s: error unmarshaling query rule: %v", logTag, err)
			continue
		}
		rules <- &rule
	}
}

func (es *elasticsearch) indexExists(ctx context.Context, indexName string) (bool, error) {
	exists, err := es.client.IndexExists(indexName).
		Do(ctx)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (es *elasticsearch) getIndexRuleWithID(ctx context.Context, indexName, ruleID string) ([]byte, error) {
	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	response, err := es.client.Get().
		Index(indexName).
		Type(es.typeName).
		Id(ruleID).
		FetchSourceContext(elastic.NewFetchSourceContext(true).
			Exclude("query")).
		Do(ctx)
	if err != nil {
		return nil, err
	}

	return json.Marshal(response.Source)
}

func (es *elasticsearch) deleteIndexRules(ctx context.Context, indexName string) (bool, error) {
	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	response, err := es.client.DeleteIndex(indexName).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return response.Acknowledged, nil
}

func (es *elasticsearch) deleteIndexRuleWithID(ctx context.Context, indexName, ruleID string) (bool, error) {
	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	_, err := es.client.Delete().
		Index(indexName).
		Type(es.typeName).
		Id(ruleID).
		Do(ctx)
	if err != nil {
		return false, err
	}

	return true, nil
}
