package pipelines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

// PipelineVar will contain the pipeline variables
// details in a proper structure
type PipelineVar struct {
	ID          *string             `json:"id" jsonschema:"title=Variable ID" jsonschema_description:"Unique Identifier for the Global variable."`
	Label       *string             `json:"label" jsonschema:"title=Label" jsonschema_description:"Name of the global variable for reference and to be shown in the UI."`
	Key         *string             `json:"key" jsonschema:"title=Key" jsonschema_description:"Key of the global variable. This key can be used to use this global variable in the pipeline."`
	Value       *string             `json:"value" jsonschema:"title=Value,description=Value of the global variable. This can be considered the most important field of the global variable since this will contain the value of the global variable."`
	Description *string             `json:"description,omitempty" jsonschema:"title=Description,description=Description of the global variable to indicate what exactly this variable is for."`
	Validate    PipelineVarValidate `json:"validate" jsonschema:"title=Validate" jsonschema_description:"To validate the entered value of the global variable. This field can be used to validate the value entered for the current variable by following the specified validators."`
	CreatedAt   *int64              `json:"created_at,omitempty" jsonschema:""`
	UpdatedAt   *int64              `json:"updated_at,omitempty"`
}

type PipelineVarValidate struct {
	URL            *string                `json:"url" jsonschema:"title=URL" jsonschema_description:""`
	Method         *string                `json:"method" jsonschema:"title=Method" jsonschema_description:""`
	Body           interface{}            `json:"body" jsonschema:"title=Body" jsonschema_description:""`
	Headers        map[string]interface{} `json:"headers" jsonschema:"title=Headers" jsonschema_description:""`
	ExpectedStatus int                    `json:"expected_status" jsonschema:"title=Expected Status" jsonschema_description:""`
}

// cachedVars represents the vars present in the cluster
// It will be the source of truth for accessing vars in the pipeline
// It should always be updated first during var creation/update.
// During startup these vars will be synced.
var cachedVars []PipelineVar

// ToMap will convert the interface to a map[string]interface{} using the
// properties present.
func (pipelineVar PipelineVar) ToMap() (map[string]interface{}, error) {
	// Marshal the pipeline into bytes
	varAsBytes, err := json.Marshal(pipelineVar)
	if err != nil {
		return nil, err
	}

	varAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(varAsBytes, &varAsMap)
	if unmarshalErr != nil {
		return nil, err
	}

	return varAsMap, nil
}

// createVar will add a new var to the index
func (es *varElasticsearch) createVar(ctx context.Context, varId string, varDoc PipelineVar) error {
	// Add the var in the index
	_, err := util.GetClient7().
		Index().
		Index(es.indexName).
		BodyJson(varDoc).
		Refresh("wait_for").
		Id(varId).
		Do(ctx)

	if err != nil {
		log.Warnln(logTag, ": error while adding variable to index: ", err)
		return err
	}

	return nil
}

// updateVar will update the var in the index with the new contents
func (es *varElasticsearch) updateVar(ctx context.Context, varId string, varDoc PipelineVar) error {
	// Add the var to the index
	_, err := util.GetClient7().Update().Index(es.indexName).Id(varId).Doc(varDoc).Do(ctx)

	if err != nil {
		log.Warnln(logTag, ": error while updating variable in index, ", err)
		return err
	}

	return nil
}

// deleteVar will remove the var from the index
func (es *varElasticsearch) deleteVar(ctx context.Context, varId string, key string) error {
	// Remove the var from the index
	_, err := util.GetClient7().
		Delete().
		Index(es.indexName).
		Id(varId).
		Do(ctx)

	if err != nil {
		log.Warnln(logTag, ": error while deleting the variable from index, ", err)
		return err
	}

	// Update the cache
	_, location := GetVarAndLocation(key)
	if location == nil {
		errMsg := fmt.Sprintf("%s: ID not present in variable cache", varId)
		log.Warnln(logTag, errMsg)
		return errors.New(errMsg)
	}

	return nil
}

// getVars will get the vars from the index and return them as an array
func (es *varElasticsearch) getVars(ctx context.Context) ([]PipelineVar, error) {
	response, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Size(int(getSizeLimitByPlan())).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving variables record:", err)
		return nil, err
	}

	var final []PipelineVar

	// NOTE: The following should not happen anymore with the es7 client either.
	// On version 8, then the response object will not have a valid Hits value
	// so it will end up with an out of memory error.
	if response.Hits == nil || response.Hits.Hits == nil {
		return final, nil
	}

	for _, hit := range response.Hits.Hits {
		var pipelineVar PipelineVar
		err := json.Unmarshal(hit.Source, &pipelineVar)
		if err != nil {
			log.Warnln(logTag, ": error while marshalling pipeline record: ", err)
			continue
		}

		final = append(final, pipelineVar)
	}

	return final, nil
}

// Define other cache methods

// GetVarAndLocation will get the variable and the location for it
// from the cache
func GetVarAndLocation(key string) (*PipelineVar, *int) {
	for location, pipelineVar := range cachedVars {
		if *pipelineVar.Key == key {
			return &pipelineVar, &location
		}
	}

	return nil, nil
}

// GetVars will get the vars and return them in terms of updatedAt
// where the latest will be the first
func GetVars() []PipelineVar {
	sort.Slice(cachedVars, func(i, j int) bool {
		if cachedVars[i].UpdatedAt != nil && cachedVars[j].UpdatedAt != nil {
			return *cachedVars[i].UpdatedAt < *cachedVars[j].UpdatedAt
		}
		return false
	})
	return cachedVars
}

// AddVar will add the passed var to cache
func AddVar(varDoc PipelineVar) {
	cachedVars = append(cachedVars, varDoc)
}

// SetVars will set the passed pipeline vars in the cache
func SetVars(varsToCache []PipelineVar) {
	cachedVars = varsToCache
}

// RemoveVar will remove the variable from the cache
func RemoveVar(location int) {
	cachedVars = append(cachedVars[:location], cachedVars[location+1:]...)
}

// UpdateVar will update the variable in cache with the
// passed location
func UpdateVar(location int, varDoc PipelineVar) {
	cachedVars[location] = varDoc
}

// Inject envs will inject all the cachedVars into the
// passed map and return them.
func InjectEnvs(passedEnvs *map[string]interface{}) {
	log.Debug(logTag, "injecting global vars into passed envs map")
	for _, pipelineVar := range cachedVars {
		envKey, envValue := *pipelineVar.Key, *pipelineVar.Value

		// If the key already exists, don't add it again
		// This is because we need to respect the user passed
		// variables that are passed explicitly.
		_, isPresent := (*passedEnvs)[envKey]
		if isPresent {
			log.Debugln(logTag, fmt.Sprintf("skipping `%s` since it's already present", envKey))
			continue
		}

		log.Debugln(logTag, fmt.Sprintf("injecting %s with value: %s", envKey, envValue))
		(*passedEnvs)[envKey] = envValue
	}
}
