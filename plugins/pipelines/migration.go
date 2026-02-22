package pipelines

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

// Mapping for the .pipelines index
const pipelinesMapping = `{
	"properties": {
		"stages": {
			"properties": {
				"script": {
					"type": "binary"
				}
			}
		}
	}
}`

const pipelineLogsMapping = `{
	"properties": {
		"@timestamp": {
			"type": "date"
		},
		"timestamp":{
			"type":"date"
		}
	}
}`

const pipelineVarMapping = `{}`

// Mapping for .pipeline_invocations index
const pipelineInvocationMapping = `{
	"properties": {
		"pipeline_id": {
			"type": "keyword"
		},
		"timestamp": {
			"type": "date"
		},
		"stages": {
			"type": "object"
		}
	}
}`

type MappingsMigration struct {
	indexName string
	es        *elasticsearch
}

// TODO: Update the logic in the function
func (m MappingsMigration) ConditionCheck() (bool, *util.Error) {
	return true, nil
}

func (m MappingsMigration) Script() *util.Error {
	log.Println(logTag, "Running migration script for pipeline....This process may take some time.")
	errorMsg := `Error occurred while updating pipelines mapping. 
	Try restarting once if it doesn't fix the issue then please contact us by opening an issue on the GitHub repository.`

	_, err := util.GetClient7().PutMapping().
		Index(m.indexName).
		BodyString(pipelinesMapping).
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
