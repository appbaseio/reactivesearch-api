package analytics

import (
	"context"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type UserSessionMappingsMigration struct {
	NewMapping string
	es         *elasticsearch
	indexName  string
}

// Returns the user session mappings
func getUserSessionMappings() string {
	properties := `
	"properties": {
		"bounce": {
			"type": "boolean"
		},
		"duration": {
			"type": "long"
		},
		"indices": {
			"type": "text",
			"fields": {
				"keyword": {
					"type": "keyword",
					"ignore_above": 256
				}
			}
		},
		"last_interaction_time": {
			"type": "long"
		},
		"start_time": {
			"type": "long"
		},
		"timestamp": {
			"type": "date"
		},
		"user_id": {
			"type": "text",
			"fields": {
				"keyword": {
					"type": "keyword",
					"ignore_above": 256
				}
			}
		},
		"custom_events": {
			"type": "nested"
        }
	}
	`
	switch util.GetVersion() {
	case 6:
		return fmt.Sprintf(`{ "_doc":  { %s } }`, properties)
	default:
		return fmt.Sprintf(`{ %s }`, properties)
	}
}

func (u UserSessionMappingsMigration) ConditionCheck() (bool, *util.Error) {
	errorMsg := `Error occurred while checking condition for user sessions mappings update. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`
	// Only run migration script when custom_events field is not defined in mappings
	response, err := util.GetIndexMapping(u.indexName, context.Background())

	if err != nil {
		log.Errorln(logTag, ":", err)
		return false, &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}
	var properties map[string]interface{}
	if u.indexName != "" {
		if response[u.indexName] != nil && response[u.indexName].(map[string]interface{})["mappings"] != nil {
			switch util.GetVersion() {
			case 6:
				if response[u.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"] != nil && response[u.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"] != nil {
					properties = response[u.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["_doc"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			default:
				if response[u.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"] != nil {
					properties = response[u.indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"].(map[string]interface{})
				}
			}
		}
	}
	shouldRunMigrationScript := true

	// Check nested mappings for custom_events
	if properties != nil && properties["custom_events"] != nil {
		customEventsAsMap, ok := properties["custom_events"].(map[string]interface{})
		if !ok {
			// Run the migration script
			return true, nil
		}
		if customEventsAsMap["type"] == "nested" {
			// Mappings is already updated no need to run the script
			shouldRunMigrationScript = false
		}
	}

	return shouldRunMigrationScript, nil
}

func (u UserSessionMappingsMigration) Script() *util.Error {
	log.Println(logTag, "Running migration script for user sessions index....This process may take some time.")
	errorMsg := `Error occurred while re-indexing the user sessions index mapping. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`
	_, err2 := util.GetClient7().PutMapping().
		Index(u.indexName).
		BodyString(`{
			"properties": {
			  "custom_events": {
				"type": "nested",
				"properties": {}
			  }
			}
		  }`).
		Do(context.Background())
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return &util.Error{
			Message: errorMsg,
			Err:     err2,
		}
	}
	log.Println(logTag, "Migrated user session index successfully.")
	return nil
}

func (m UserSessionMappingsMigration) IsAsync() bool {
	return false
}
