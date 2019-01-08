package rules

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/appbaseio-confidential/arc/plugins/rules/query"
	"github.com/appbaseio-confidential/arc/util"
	"github.com/gorilla/mux"
)

func (r *Rules) postRule() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indexName := mux.Vars(req)["index"]

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error reading request body", http.StatusBadRequest)
			return
		}

		var rule query.Rule
		err = json.Unmarshal(body, &rule)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error parsing request body", http.StatusBadRequest)
			return
		}

		// construct the percolator query to be indexed alongside the query rules
		err = r.withQuery(&rule)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// fetch and construct the required payload before indexing the rule
		err = r.withPayload(indexName, &rule)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// index the rule
		ok, err := r.es.postRule(req.Context(), indexName, rule)
		if !ok || err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error creating the query rule", http.StatusInternalServerError)
			return
		}

		util.WriteBackMessage(w, "Rule created", http.StatusCreated)
	}
}

func (r *Rules) withQuery(rule *query.Rule) error {
	var err error
	// construct the percolator query to be indexed alongside the query rules
	switch rule.Condition.Operator {
	case query.Is:
		{
			rule.Query.Regexp.Pattern = rule.Condition.Pattern
		}
	case query.StartsWith:
		{
			pattern := fmt.Sprintf("%s.*", rule.Condition.Pattern)
			rule.Query.Regexp.Pattern = pattern
		}
	case query.EndsWith:
		{
			pattern := fmt.Sprintf(".+%s", rule.Condition.Pattern)
			rule.Query.Regexp.Pattern = pattern
		}
	case query.Contains:
		{
			pattern := fmt.Sprintf(".*%s.*", rule.Condition.Pattern)
			rule.Query.Regexp.Pattern = pattern
		}
	default:
		err = fmt.Errorf("unhandled rule operator")
	}
	return err
}

func (r *Rules) withPayload(indexName string, rule *query.Rule) error {
	var err error
	switch rule.Consequence.Operation {
	case query.Promote:
		{
			var docIDs []string
			for _, payload := range rule.Consequence.Payload {
				docIDs = append(docIDs, payload.DocID)
			}
			indexDocs := r.fetchDocs(context.Background(), indexName, docIDs...)
			fmt.Println(indexDocs)
			for i, payload := range rule.Consequence.Payload {
				rule.Consequence.Payload[i].Doc = indexDocs[payload.DocID]
			}
		}
	case query.Inject:
		// In case of query.Inject we expect the client to provide the "doc_id" and "doc"
		// that it intends to inject along with the search result.
	case query.Hide:
		// In case of query.Hide we do not need to fetch the docs at all, instead we
		// just remove the docs with the provided ids from the search result.
	default:
		err = fmt.Errorf("unhandled rule operation")
	}

	return err
}

func (r *Rules) getIndexRules() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indexName := mux.Vars(req)["index"]

		raw, err := r.es.getIndexRules(req.Context(), indexName)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "rules for index not found", http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

type indexDoc struct {
	id, doc string
}

func (r *Rules) fetchDocs(ctx context.Context, indexName string, docIDs ...string) map[string]string {
	docs := make(chan *indexDoc)
	var wg sync.WaitGroup

	for _, docID := range docIDs {
		wg.Add(1)
		go r.es.fetchDoc(ctx, indexName, docID, docs, &wg)
	}

	go func() {
		wg.Wait()
		close(docs)
	}()

	result := make(map[string]string)
	for doc := range docs {
		if doc == nil {
			continue
		}
		result[doc.id] = doc.doc
	}

	return result
}
