package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type MappingsMigration struct {
	NewMapping string
	es         *elasticsearch
}

// Check whether or not the script should run.
//
// Since we don't have any conditions to check, we will
// return `true` directly.
func (m MappingsMigration) ConditionCheck() (bool, *util.Error) {
	errorMsg := `Error occurred while checking condition for openAI mappings update. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`

	response, err := util.GetIndexMapping(m.es.indexName, context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return false, &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}

	indexName := m.es.indexName
	if indexName == "" {
		return true, nil
	}

	if response[indexName] != nil && response[indexName].(map[string]interface{})["mappings"] != nil && response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"] != nil {
		properties := response[indexName].(map[string]interface{})["mappings"].(map[string]interface{})["properties"].(map[string]interface{})
		if properties["azureVersion"] != nil {
			currentType := properties["azureVersion"].(map[string]interface{})["type"]
			return currentType != nil && currentType != "text", nil
		}
	}

	return true, nil
}

// Script to run for the migration
func (m MappingsMigration) Script() *util.Error {
	log.Println(logTag, "Running migration script for openAI....This process may take some time.")
	errorMsg := `Error occurred while re-indexing the openAI index mapping. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`

	// Unmarshal the new mapping for creating the new index.
	var mappingAsMap map[string]interface{}
	err := json.Unmarshal([]byte(m.NewMapping), &mappingAsMap)
	if err != nil {
		log.Errorln(logTag, ":", err)
		return &util.Error{
			Message: errorMsg,
			Err:     err,
		}
	}

	// Extract the current openAI config value
	// Delete the index
	// Re-create it and add the older config value.
	openAIInstance := Instance()
	currentConfig := openAIInstance.GetConfig()

	sourceIndex := m.es.indexName

	// delete openai index
	_, err4 := util.GetClient7().DeleteIndex(sourceIndex).Do(context.Background())
	if err4 != nil {
		log.Errorln(logTag, ":", err4)
		return &util.Error{
			Message: errorMsg,
			Err:     err4,
		}
	}

	// Re-create the index.
	_, err = util.GetClient7().CreateIndex(sourceIndex).Body(m.NewMapping).Do(context.Background())
	if err != nil {
		return &util.Error{
			Message: errorMsg,
			Err:     fmt.Errorf("error while creating index named %s: %v", sourceIndex, err),
		}
	}

	// Index the document
	saveErr := openAIInstance.es.saveSettings(currentConfig.ToExternalConfig().ToInternalConfig(), context.Background())
	if saveErr != nil {
		errMsg := fmt.Sprint("error while saving passed body to index: ", saveErr.Error())
		return &util.Error{
			Message: errMsg,
			Err:     fmt.Errorf(errMsg),
		}
	}

	return nil
}

func (m MappingsMigration) IsAsync() bool {
	return false
}
