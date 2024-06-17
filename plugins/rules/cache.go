package rules

import (
	"encoding/base64"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
)

// cachedRules represents the struct of a list of saved rules in the .rules index
var cachedRules []ESRuleDoc

// SetRulesToCache sets the rules
func SetRulesToCache(rules []ESRuleDoc) {
	rulesToCache := []ESRuleDoc{}
	for _, rule := range rules {
		rulesToCache = append(rulesToCache, decodeScript(rule))
	}
	cachedRules = rulesToCache
}

// GetRulesFromCache returns a list of cached rules
func GetRulesFromCache() []ESRuleDoc {
	sort.Slice(cachedRules, func(i, j int) bool {
		return *cachedRules[i].Order < *cachedRules[j].Order
	})
	return cachedRules
}

// AddRuleToCache adds a rule to cache
func AddRuleToCache(rule ESRuleDoc) {
	cachedRules = append(cachedRules, decodeScript(rule))
}

// IsRuleExistsInCache checks if a rule in present in cache
func IsRuleExistsInCache(ruleID string) (*ESRuleDoc, *int) {
	for loc, rule := range cachedRules {
		if *rule.ID == ruleID {
			return &rule, &loc
		}
	}
	return nil, nil
}

// DeleteRuleToCache deletes a rule from cache
func DeleteRuleToCache(ruleID string) bool {
	_, loc := IsRuleExistsInCache(ruleID)
	if loc != nil {
		cachedRules = append(cachedRules[:*loc], cachedRules[*loc+1:]...)
		return true
	}
	return false
}

// GetRuleFromCache returns a rule by ID
func GetRuleFromCache(ruleID string) *ESRuleDoc {
	rule, _ := IsRuleExistsInCache(ruleID)
	return rule
}

// UpdateRuleToCache updates a rule without overriding the previous values
func UpdateRuleToCache(ruleID string, rule ESRuleDoc) bool {
	savedRule, loc := IsRuleExistsInCache(ruleID)
	if ruleID != "" && loc != nil {
		trigger := savedRule.Trigger
		if rule.Trigger != nil {
			trigger = rule.Trigger
		}
		order := savedRule.Order
		if rule.Order != nil {
			order = rule.Order
		}
		actions := savedRule.Actions
		if rule.Actions != nil {
			actions = rule.Actions
		}

		updatedAt := rule.UpdatedAt
		if updatedAt == nil {
			currentTime := time.Now().Unix()
			updatedAt = &currentTime
		}

		enabled := savedRule.Enabled
		if rule.Enabled != nil {
			enabled = rule.Enabled
		}

		name := savedRule.Name
		if rule.Name != nil {
			name = rule.Name
		}

		description := savedRule.Description
		if rule.Description != nil {
			description = rule.Description
		}

		showAdvanceEditor := savedRule.ShowAdvanceEditor
		if rule.ShowAdvanceEditor != nil {
			showAdvanceEditor = rule.ShowAdvanceEditor
		}

		cachedRules[*loc] = decodeScript(ESRuleDoc{
			ID:                &ruleID,
			Name:              name,
			Description:       description,
			Enabled:           enabled,
			Order:             order,
			Trigger:           trigger,
			Actions:           actions,
			UpdatedAt:         updatedAt,
			CreatedAt:         savedRule.CreatedAt,
			ShowAdvanceEditor: showAdvanceEditor,
		})
		return true
	}
	return false
}

func decodeScript(rule ESRuleDoc) ESRuleDoc {
	if rule.Actions != nil {
		actions := *rule.Actions
		for i, action := range actions {
			if action.Type != nil && *action.Type == Script {
				if action.Script != nil {
					script, err := base64.StdEncoding.DecodeString(*action.Script)
					if err != nil {
						log.Errorln(logTag, ":", err)
					} else {
						scriptAsString := string(script)
						actions[i].DecodeScript = &scriptAsString
					}
				}
			}
		}
		*(rule.Actions) = actions
	}
	return rule
}

func removeScriptFromRule(rule ESRuleRequestBody) ESRuleRequestBody {
	if rule.Actions != nil {
		actions := *rule.Actions
		for i, action := range actions {
			if action.Type != nil && *action.Type == Script {
				actions[i].Script = rule.ID
			}
		}
		*(rule.Actions) = actions
	}
	return rule
}
