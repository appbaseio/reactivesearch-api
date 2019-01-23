package rules

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/index"
	"github.com/appbaseio-confidential/arc/plugins/rules/query"
	"github.com/appbaseio-confidential/arc/util"
)

func Apply() middleware.Middleware {
	return Instance().intercept
}

// Intercept middleware intercepts the search requests and applies query rules to the search results.
// TODO: Define middleware chain for rules plugin
func (r *Rules) intercept(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		c, err := category.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error occurred while processing request", http.StatusInternalServerError)
			return
		}

		indices, err := index.FromContext(ctx)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error occurred while processing request", http.StatusInternalServerError)
			return
		}

		queryTerm := req.Header.Get("X-Search-Query")
		if queryTerm == "" || len(indices) == 0 || *c != category.Search {
			h(w, req)
			return
		}

		rules := make(chan *query.Rule)
		go r.es.fetchQueryRules(ctx, indices[0], queryTerm, rules)

		resp := httptest.NewRecorder()
		h(resp, req)

		result := resp.Result()
		body, err := ioutil.ReadAll(result.Body)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error reading response body", http.StatusInternalServerError)
			return
		}

		var searchResult map[string]interface{}
		err = json.Unmarshal(body, &searchResult)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error unmarshaling search result", http.StatusInternalServerError)
			return
		}

		for rule := range rules {
			if err = applyRule(searchResult, rule); err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, "error applying rules to search result", http.StatusInternalServerError)
				return
			}
		}

		raw, err := json.Marshal(searchResult)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error marshaling search result", http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func applyRule(searchResult map[string]interface{}, rule *query.Rule) error {
	var err error
	switch rule.Then.Action {
	case query.Promote:
		var promotedResults []interface{}
		for _, payload := range rule.Then.Payloads {
			promotedResults = append(promotedResults, payload.Doc)
		}
		searchResult["promoted"] = promotedResults

	// TODO: modify this ugly workaround
	case query.Hide:
		totalHits, ok := searchResult["hits"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unable to cast search hits to map[string]interface{}")
		}
		hits, ok := totalHits["hits"].([]interface{})
		if !ok {
			return fmt.Errorf("unable to cast hits.hits to []interface{}")
		}

		for _, payload := range rule.Then.Payloads {
			for j, h := range hits {
				hit, ok := h.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unable to cast hit to map[string]interface{}")
				}
				if hit["_id"] != nil && payload.DocID == fmt.Sprintf("%v", hit["_id"]) {
					hits = append(hits[:j], hits[j+1:]...)
				}
			}
		}

		totalHits["hits"] = hits
		totalHits["total"] = len(hits)
		searchResult["hits"] = totalHits
	default:
		err = fmt.Errorf("unhandled then action")
	}
	return err
}
