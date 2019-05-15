package rules

import (
	"bytes"
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
	"github.com/appbaseio-confidential/arc/plugins/logs"
	"github.com/appbaseio-confidential/arc/plugins/rules/query"
	"github.com/appbaseio-confidential/arc/util"

	"github.com/siddharthlatest/mustache"
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
		logs.Recorder(),
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

	// handle the webhook if there is one
	if rule.Then.WebHook != nil {
		if err := handleWebHook(searchResult, rule); err != nil {
			return err
		}
	}

	return nil
}

func handleWebHook(searchResult map[string]interface{}, rule *query.Rule) error {
	// the webhook payload template can either be a string, or a json object
	// the json object has to be a string -> string mapping

	// if it's a string, then the payload body will be a string
	// if it's a JSON object, then the payload body will be a JSON object

	// payloadBytes is what we would be sending to the webhook
	var err error
	var payloadBytes []byte

	// if searchResult or payload template is nil, there is no need to construct the payload
	if searchResult != nil && rule.Then.WebHook.PayloadTemplate != nil {
		switch v := rule.Then.WebHook.PayloadTemplate.(type) {
		case string:
			payload, err := mustache.Render(v, searchResult["hits"])
			if err != nil {
				return err
			}

			payloadBytes = []byte(payload)

		case map[string]interface{}:
			payload := map[string]string{}
			for key, template := range v {
				templateString, ok := template.(string)
				if !ok {
					return fmt.Errorf("the values of the webhook payload json object must be strings")
				}
				payload[key], err = mustache.Render(templateString, searchResult["hits"])
				if err != nil {
					return err
				}
			}

			payloadBytes, err = json.Marshal(payload)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("payload template of webhook needs to be a string or a json object")
		}
	}

	// create a http client
	httpClient := &http.Client{}

	// construct the request
	// pass the marshalled search results as body
	req, err := http.NewRequest(http.MethodGet, rule.Then.WebHook.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	// apply the headers if any
	for k, v := range rule.Then.WebHook.Headers {
		req.Header.Set(k, v)
	}

	// call the webhook
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// return immediately if search results aren't to be overwritten
	if !rule.Then.WebHook.OverwriteSearchResults {
		return nil
	}

	// unmarshal search results into list of objects
	// assign them to searchResults["hits"]["hits"]
	respArray := []interface{}{}

	if err := json.NewDecoder(resp.Body).Decode(&respArray); err != nil {
		return err
	}

	searchResult["hits"].(map[string]interface{})["hits"] = respArray

	return nil
}
