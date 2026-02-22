package rules

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type MappingsMigration struct {
	indexName string
	es        *elasticsearch
}

const rulesMapping = `{
	"properties": {
		"actions": {
			"properties": {
				"script": {
					"type": "binary"
				}
			}
		}
	}
}`

func (m MappingsMigration) ConditionCheck() (bool, *util.Error) {
	errorMsg := `Error occurred while checking condition for rules mappings update. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`
	// Only run migration script when mapping is not present for `script`
	response, err := util.GetIndexMapping(m.indexName, context.Background())

	if err != nil {
		log.Errorln(logTag, ":", err)
		return false, &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	var properties map[string]interface{}
	if m.indexName != "" {
		if response[m.indexName] != nil && response[m.indexName].(map[string]interface{})["mappings"] != nil {
			switch util.GetVersion() {
			case 6:
				if response[m.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"] != nil && response[m.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"] != nil {
					properties = response[m.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			default:
				if response[m.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"] != nil {
					properties = response[m.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			}
		}
	}
	shouldRunMigrationScript := true

	// Check nested mappings for script
	if properties != nil && properties["actions"] != nil {
		searchFiltersAsMap, ok := properties["actions"].(map[string]interface{})["properties"].(map[string]interface{})["script"].(map[string]interface{})
		if !ok {
			// Run the migration script
			return true, nil
		}
		if searchFiltersAsMap["type"] == "binary" {
			// Mappings is already updated no need to run the script
			shouldRunMigrationScript = false
		} else {
			// Run the migration script
			return true, nil
		}
	}

	return shouldRunMigrationScript, nil
}

func (m MappingsMigration) Script() *util.Error {
	log.Println(logTag, "Running migration script for rules....This process may take some time.")
	errorMsg := `Error occurred while updating rules mapping. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`

	_, err := util.GetClient7().PutMapping().
		Index(m.indexName).
		BodyString(rulesMapping).
		Do(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	return nil
}

func (m MappingsMigration) IsAsync() bool {
	return false
}
