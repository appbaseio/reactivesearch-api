package rules

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
	"github.com/appbaseio-confidential/arc/middleware/classify"
	"github.com/appbaseio-confidential/arc/middleware/validate"
	"github.com/appbaseio-confidential/arc/model/category"
	"github.com/appbaseio-confidential/arc/model/index"
	"github.com/appbaseio-confidential/arc/plugins/auth"
	"github.com/appbaseio-confidential/arc/plugins/rules/query"
	"github.com/appbaseio-confidential/arc/util"
)

type chain struct {
	order.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{
		classifyCategory,
		classifyIndices,
		classify.Op(),
		auth.BasicAuth(),
		validate.Operation(),
		validate.Category(),
	}
}

func classifyCategory(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		userCategory := category.User

		ctx := category.NewContext(req.Context(), &userCategory)
		req = req.WithContext(ctx)

		h(w, req)
	}
}

func classifyIndices(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx := index.NewContext(req.Context(), []string{defaultRulesEsIndex})
		req = req.WithContext(ctx)
		h(w, req)
	}
}

// Apply middleware intercepts the search requests and applies query rules to the search results.
func Apply() middleware.Middleware {
	return Instance().intercept
}

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
	// apply promote action by appending the payload
	if rule.Then.Promote != nil {
		searchResult["promoted"] = rule.Then.Promote
	}

	// apply hide action
	if rule.Then.Hide != nil {
		totalHits, ok := searchResult["hits"].(map[string]interface{})
		if !ok {
			return fmt.Errorf("unable to cast search hits to map[string]interface{}")
		}
		hits, ok := totalHits["hits"].([]interface{})
		if !ok {
			return fmt.Errorf("unable to cast hits.hits to []interface{}")
		}

		for _, doc := range rule.Then.Hide {
			for i, hit := range hits {
				hit, ok := hit.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unable to cast hit to map[string]interface{}")
				}
				if hit["_id"] != nil && *doc.DocID == fmt.Sprintf("%v", hit["_id"]) {
					hits = append(hits[:i], hits[i+1:]...)
				}
			}
		}
		totalHits["hits"] = hits
		totalHits["total"] = len(hits)
		searchResult["hits"] = totalHits
	}

	return nil
}
