package rules

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/index"
	"github.com/appbaseio-confidential/arc/plugins/rules/query"
	"github.com/appbaseio-confidential/arc/util"
)

type injectedResult struct {
	DocID string `json:"doc_id"`
	Doc   string `json:"doc"`
}

type promotedResult struct {
	DocID string `json:"doc_id"`
	Doc   string `json:"doc"`
}

type esSearchResult struct {
	Took     int  `json:"took"`
	TimedOut bool `json:"timed_out"`
	Shards   struct {
		Total      int `json:"total"`
		Successful int `json:"successful"`
		Skipped    int `json:"skipped"`
		Failed     int `json:"failed"`
	} `json:"_shards"`
	Hits struct {
		Total    int     `json:"total"`
		MaxScore float64 `json:"max_score"`
		Hits     []struct {
			Index  string      `json:"_index"`
			Type   string      `json:"_type"`
			ID     string      `json:"_id"`
			Score  float64     `json:"_score"`
			Source interface{} `json:"_source"`
		} `json:"hits,omitempty"`
	} `json:"hits"`
	Injected []injectedResult `json:"injected,omitempty"`
	Promoted []promotedResult `json:"promoted,omitempty"`
}

// Intercept middleware intercepts the search requests and applyies query rules to the search results.
// TODO: Define middleware chain for rules plugin
func (r *Rules) Intercept(h http.HandlerFunc) http.HandlerFunc {
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

		rules := make(chan query.Rule)
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

		var searchResult esSearchResult
		err = json.Unmarshal(body, &searchResult)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error unmarshaling search result", http.StatusInternalServerError)
			return
		}

		for rule := range rules {
			fmt.Println(rule)
			applyRule(&searchResult, rule)
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

func applyRule(searchResult *esSearchResult, rule query.Rule) {
	switch rule.Consequence.Operation {
	case query.Inject:
		{
			var injectedResponses []injectedResult
			for _, payload := range rule.Consequence.Payload {
				var injectedResponse = *new(injectedResult)
				injectedResponse.DocID = payload.DocID
				injectedResponse.Doc = payload.Doc
				injectedResponses = append(injectedResponses, injectedResponse)
			}
			searchResult.Injected = injectedResponses
		}
	case query.Promote:
		{
			var promotedResults []promotedResult
			for _, payload := range rule.Consequence.Payload {
				var promotedResult = *new(promotedResult)
				promotedResult.DocID = payload.DocID
				promotedResult.Doc = payload.Doc
				promotedResults = append(promotedResults, promotedResult)
			}
			searchResult.Promoted = promotedResults
		}
	case query.Hide:
		{
			for _, payload := range rule.Consequence.Payload {
				for j, hit := range searchResult.Hits.Hits {
					if payload.DocID == hit.ID {
						searchResult.Hits.Hits =
							append(searchResult.Hits.Hits[:j], searchResult.Hits.Hits[j+1:]...)
					}
				}
			}
		}
	}
}
