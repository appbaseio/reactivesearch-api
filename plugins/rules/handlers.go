package rules

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/appbaseio-confidential/arc/plugins/rules/query"
	"github.com/appbaseio-confidential/arc/util"
	"github.com/gorilla/mux"
)

func (r *Rules) postIndexRule() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indexName := mux.Vars(req)["index"]

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error reading request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var rule query.Rule
		err = json.Unmarshal(body, &rule)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error parsing request body", http.StatusBadRequest)
			return
		}

		// validate the rule provided by the user
		if err = validateRule(&rule); err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusBadRequest)
			return
		}

		// set the "id" field to random string, if not assigned by the user
		if rule.ID == "" {
			// e.g: starts_with_apple, is_apple, ends_with_apple, contains_apple
			rule.ID = fmt.Sprintf("%s_%s", rule.If.Operator, *rule.If.Query)
		}
		response, _ := json.Marshal(rule)

		// construct the percolator query to be indexed alongside the query rules
		if err = withQuery(&rule); err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if ok, err := r.es.postIndexRule(req.Context(), indexName, &rule); !ok || err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error creating the query rule", http.StatusInternalServerError)
			return
		}

		util.WriteBackRaw(w, response, http.StatusCreated)
	}
}

func (r *Rules) postIndexRules() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indexName := mux.Vars(req)["index"]

		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error reading request body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		var rules, rulesWithID []query.Rule
		err = json.Unmarshal(body, &rules)
		if err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error parsing request body", http.StatusBadRequest)
			return
		}

		for i := range rules {
			// validate the rule provided by the user
			if err = validateRule(&rules[i]); err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusBadRequest)
				return
			}

			// set the "id" field to random string, if not assigned by the user
			if rules[i].ID == "" {
				// e.g: starts_with_apple, is_apple, ends_with_apple, contains_apple
				rules[i].ID = fmt.Sprintf("%s_%s", rules[i].If.Operator, *rules[i].If.Query)
			}
			rulesWithID = append(rulesWithID, rules[i])

			// construct the percolator query to be indexed alongside the query rules
			if err = withQuery(&rules[i]); err != nil {
				log.Printf("%s: %v", logTag, err)
				util.WriteBackError(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		if ok, err := r.es.postIndexRules(req.Context(), indexName, rules); !ok || err != nil {
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, "error creating the query rule", http.StatusInternalServerError)
			return
		}

		response, _ := json.Marshal(rulesWithID)
		util.WriteBackRaw(w, response, http.StatusCreated)
	}
}

func withQuery(rule *query.Rule) error {
	// construct the percolator query to be indexed alongside the query rules
	rule.Query = new(query.Query)
	var err error
	switch *rule.If.Operator {
	case query.Is:
		rule.Query.Regexp.Pattern = *rule.If.Query
	case query.StartsWith:
		pattern := fmt.Sprintf("%s.*", *rule.If.Query)
		rule.Query.Regexp.Pattern = pattern
	case query.EndsWith:
		pattern := fmt.Sprintf(".+%s", *rule.If.Query)
		rule.Query.Regexp.Pattern = pattern
	case query.Contains:
		pattern := fmt.Sprintf(".*%s.*", *rule.If.Query)
		rule.Query.Regexp.Pattern = pattern
	default:
		err = fmt.Errorf("unhandled rule operator")
	}
	return err
}

func validateRule(rule *query.Rule) error {
	// "query" is an internal field and cannot be set by the user.
	if rule.Query != nil {
		return fmt.Errorf(`field "query" cannot be set externally`)
	}

	// "if" field should not be nil
	if rule.If == nil {
		return fmt.Errorf(`field "if" cannot be set to nil`)
	}

	// "if.query" should not be nil
	if rule.If.Query == nil {
		return fmt.Errorf(`field "if.query" cannot be set to nil`)
	}

	// "if.operator" should not be nil
	if rule.If.Operator == nil {
		return fmt.Errorf(`field "if.operator" cannot be set to nil`)
	}

	// "then" should not be nil
	if rule.Then == nil {
		return fmt.Errorf(`field "then" cannot be set to nil`)
	}

	// "then.promote" should not be empty
	if rule.Then.Promote != nil && len(rule.Then.Promote) == 0 {
		return fmt.Errorf(`field "then.promote" cannot be empty`)
	}

	// "then.hide" should not be empty
	if rule.Then.Hide != nil {
		if len(rule.Then.Hide) == 0 {
			return fmt.Errorf(`field "then.hide" cannot be empty`)
		}
		for _, obj := range rule.Then.Hide {
			if obj.DocID == nil {
				return fmt.Errorf(`field "doc_id" cannot be set to nil`)
			}
		}
	}

	if rule.Then.Hide == nil && rule.Then.Promote == nil {
		return fmt.Errorf(`field "then" must contain atleast one action`)
	}

	return nil
}

func (r *Rules) getIndexRules() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indexName := mux.Vars(req)["index"]

		raw, err := r.es.getIndexRules(req.Context(), indexName)
		if err != nil {
			msg := fmt.Sprintf("Rules for index=%s not found", indexName)
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (r *Rules) getIndexRuleWithID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ruleID := vars["index"], vars["id"]

		raw, err := r.es.getIndexRuleWithID(req.Context(), indexName, ruleID)
		if err != nil {
			msg := fmt.Sprintf("Rule for index=%s, id=%s not found", indexName, ruleID)
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackRaw(w, raw, http.StatusOK)
	}
}

func (r *Rules) deleteIndexRules() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		indexName := mux.Vars(req)["index"]

		_, err := r.es.deleteIndexRules(req.Context(), indexName)
		if err != nil {
			msg := fmt.Sprintf("Rules for index=%s not found", indexName)
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackMessage(w, "Deleted index rules", http.StatusOK)
	}
}

func (r *Rules) deleteIndexRuleWithID() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		vars := mux.Vars(req)
		indexName, ruleID := vars["index"], vars["id"]

		_, err := r.es.deleteIndexRuleWithID(req.Context(), indexName, ruleID)
		if err != nil {
			msg := fmt.Sprintf("Rule for index=%s, id=%s not found", indexName, ruleID)
			log.Printf("%s: %v", logTag, err)
			util.WriteBackError(w, msg, http.StatusNotFound)
			return
		}

		util.WriteBackMessage(w, "Deleted index rule", http.StatusOK)
	}
}
