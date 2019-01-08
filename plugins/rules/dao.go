package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

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
	es.indexConfig = fmt.Sprintf(mapping, nodes-1)

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

func (es *elasticsearch) postRule(ctx context.Context, indexName string, rule query.Rule) (bool, error) {
	// e.g: app-rules
	rulesIndexName := fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	// check if index already exists, this is important
	// because we need to set the percolator mapping to the
	// index before indexing the rules.
	exists, err := es.client.IndexExists(rulesIndexName).
		Do(ctx)
	if err != nil {
		return false, err
	}
	if !exists {
		if created, err := es.createIndex(ctx, rulesIndexName); err != nil || !created {
			return false, err
		}
	}

	// e.g: contains-apple, is-apple, starts-with-apple, ends-with-apple ...
	ruleID := fmt.Sprintf("%s-%s", rule.Condition.Operator, rule.Condition.Pattern)

	_, err = es.client.Index().
		Index(rulesIndexName).
		Type(es.typeName).
		Id(ruleID).
		BodyJson(rule).
		Do(ctx)
	if err != nil {
		return false, err
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

func (es *elasticsearch) fetchQueryRules(ctx context.Context, indexName, queryTerm string, rules chan<- query.Rule) {
	defer close(rules)

	indexName = fmt.Sprintf("%s-%s", indexName, es.indexSuffix)

	doc := map[string]interface{}{"condition.pattern": queryTerm}
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
		raw, _ := hit.Source.MarshalJSON()
		fmt.Println(string(raw))
		var rule query.Rule
		err := json.Unmarshal(*hit.Source, &rule)
		if err != nil {
			log.Printf("%s: error unmarshaling query rule: %v", logTag, err)
			continue
		}
		rules <- rule
	}
}

func (es *elasticsearch) fetchDoc(ctx context.Context, indexName, docID string, docs chan<- *indexDoc, wg *sync.WaitGroup) {
	defer wg.Done()

	response, err := es.client.Get().
		Index(indexName).
		Type(es.typeName).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Printf("%s: error fetching doc with id=%s from index=%s: %v", logTag, docID, indexName, err)
		return
	}

	doc, err := response.Source.MarshalJSON()
	if err != nil {
		log.Printf("%s: error marshaling doc with id=%s from index=%s: %v", logTag, docID, indexName, err)
		return
	}

	docs <- &indexDoc{docID, string(doc)}
}
