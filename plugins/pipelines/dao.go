package pipelines

import (
	"context"
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/middleware/classify"
	"github.com/appbaseio/reactivesearch-api/model/reindex"
	"github.com/appbaseio/reactivesearch-api/util"
)

type elasticsearch struct {
	indexName string
}

type invocationElasticsearch struct {
	indexName string
}

type logsElasticsearch struct {
	indexName string
}

type varElasticsearch struct {
	indexName string
}

func initPlugin(pipelinesIndex, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{pipelinesIndex}

	ctx := context.Background()

	// Check if the pipelines index already exists
	exists, err := util.GetClient7().IndexExists(pipelinesIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, pipelinesIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, pipelinesMapping, util.HiddenIndexSettings(), util.MetaIndexShards(3), replicas))

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(pipelinesIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", pipelinesIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, pipelinesIndex)
	return es, nil
}

// initInvocationIndex will initiate the index to store pipeline invocation details.
func initInvocationIndex(invocationAlias, _ string) (*invocationElasticsearch, error) {
	es := &invocationElasticsearch{invocationAlias}

	ctx := context.Background()

	// Check if alias exists instead of index and create first index if not exists with `${alias}-000001`
	res, err := util.GetClient7().Aliases().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	indices := res.IndicesByAlias(invocationAlias)
	exists := false
	if len(indices) > 0 {
		exists = true
	}

	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, invocationAlias)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(invocationConfig, invocationAlias, util.HiddenIndexSettings(), util.MetaIndexShards(3), replicas, pipelineInvocationMapping))

	// Create the index name to match the name regex for rollover
	invocationIndex := invocationAlias + `-000001`
	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(invocationIndex).Body(settings).Do(ctx)
	if err != nil {
		log.Errorln(logTag, " : ", fmt.Errorf("error while creating index named \"%s\" %v", invocationIndex, err))
		isAliasExistsAsIndex, err := util.GetClient7().IndexExists(invocationAlias).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while checking if index already exists: %v", err)
		}
		if !isAliasExistsAsIndex {
			return nil, fmt.Errorf("error while creating index named \"%s\" %v", invocationIndex, err)
		}
		// If .pipeline_invocations exists as an index then perform following steps:
		// 1. Re-index `.pipeline_invocations` to `.pipeline_invocations-000001`
		// 2. Delete `.pipeline_invocations` and continue
		sourceIndex := invocationAlias
		destinationIndex := invocationIndex
		var settingsAsMap map[string]interface{}
		err1 := json.Unmarshal([]byte(settings), &settingsAsMap)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			return nil, fmt.Errorf("error while un-marshalling invocation mappings %v", err1)
		}
		settings, _ := settingsAsMap["settings"].(map[string]interface{})
		mappings, _ := settingsAsMap["mappings"].(map[string]interface{})
		reIndexConfig := reindex.ReindexConfig{
			Settings: settings,
			Mappings: mappings,
		}
		log.Infoln(logTag, ": re-indexing ", invocationIndex, " index, this may take a while...")
		taskDetails, err := reindex.Reindex(context.Background(), sourceIndex, &reIndexConfig, false, destinationIndex)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, nil
		}
		// Re-index synchronously
		reindex.TrackReindex(reindex.SetAliasConfig{
			AliasName:    sourceIndex,
			NewIndex:     destinationIndex,
			OldIndex:     sourceIndex,
			IsWriteIndex: true,
		}, taskDetails)
	}

	classify.SetIndexAlias(invocationIndex, invocationAlias)
	classify.SetAliasIndex(invocationAlias, invocationIndex)

	log.Printf("%s successfully created index named '%s'", logTag, invocationAlias)
	return es, nil
}

// initLogIndex will initiate the logs index where the logs will be stored
func initLogIndex(logsAlias, mapping string) (*logsElasticsearch, error) {
	es := &logsElasticsearch{logsAlias}

	ctx := context.Background()

	// Check if alias exists instead of index and create first index if not exists with `${alias}-000001`
	res, err := util.GetClient7().Aliases().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	indices := res.IndicesByAlias(logsAlias)
	exists := false
	if len(indices) > 0 {
		exists = true
	}

	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, logsAlias)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, logsAlias, util.HiddenIndexSettings(), util.MetaIndexShards(3), replicas, pipelineLogsMapping))

	// Create the index name to match the name regex for
	logsIndex := logsAlias + `-000001`
	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(logsIndex).Body(settings).Do(ctx)
	if err != nil {
		log.Errorln(logTag, " : ", fmt.Errorf("error while creating index named \"%s\" %v", logsIndex, err))
		isAliasExistsAsIndex, err := util.GetClient7().IndexExists(logsAlias).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while checking if index already exists: %v", err)
		}
		if !isAliasExistsAsIndex {
			return nil, fmt.Errorf("error while creating index named \"%s\" %v", logsIndex, err)
		}
		// If .pipeline_logs exists as an index then perform following steps:
		// 1. Re-index `.logs` to `.pipeline_logs-000001`
		// 2. Delete `.pipeline_logs` and continue
		sourceIndex := logsAlias
		destinationIndex := logsIndex
		var settingsAsMap map[string]interface{}
		err1 := json.Unmarshal([]byte(settings), &settingsAsMap)
		if err1 != nil {
			log.Errorln(logTag, ":", err1)
			return nil, fmt.Errorf("error while un-marshalling logs mappings %v", err1)
		}
		settings, _ := settingsAsMap["settings"].(map[string]interface{})
		mappings, _ := settingsAsMap["mappings"].(map[string]interface{})
		reIndexConfig := reindex.ReindexConfig{
			Settings: settings,
			Mappings: mappings,
		}
		log.Infoln(logTag, ": re-indexing ", logsIndex, " index, this may take a while...")
		taskDetails, err := reindex.Reindex(context.Background(), sourceIndex, &reIndexConfig, false, destinationIndex)
		if err != nil {
			log.Errorln(logTag, ":", err)
			return nil, nil
		}
		// Re-index synchronously
		reindex.TrackReindex(reindex.SetAliasConfig{
			AliasName:    sourceIndex,
			NewIndex:     destinationIndex,
			OldIndex:     sourceIndex,
			IsWriteIndex: true,
		}, taskDetails)
	}

	classify.SetIndexAlias(logsIndex, logsAlias)
	classify.SetAliasIndex(logsAlias, logsIndex)

	log.Printf("%s successfully created index named '%s'", logTag, logsAlias)
	return es, nil
}

// initVarIndex will initiate the variable index where the vars will be stored.
func initVarIndex(varIndex, mapping string) (*varElasticsearch, error) {
	es := &varElasticsearch{varIndex}

	ctx := context.Background()

	// Check if the vars index already exists
	exists, err := util.GetClient7().IndexExists(varIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, varIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, pipelineVarMapping, util.HiddenIndexSettings(), util.MetaIndexShards(3), replicas))

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(varIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", varIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, varIndex)
	return es, nil
}

// To create pipeline in ES index
func (es *elasticsearch) createPipeline(ctx context.Context, pipelineID string, record ESPipelineDoc) error {
	return es.createPipelineEs7(ctx, pipelineID, record)
}

// To update a pipeline in ES index
func (es *elasticsearch) updatePipeline(ctx context.Context, pipelineID string, record ESPipelineDoc) error {
	return es.updatePipelineEs7(ctx, pipelineID, record)
}

// To delte a pipeline in ES index
func (es *elasticsearch) deletePipeline(ctx context.Context, pipelineID string) error {
	return es.deletePipelineEs7(ctx, pipelineID)
}

// To get pipelines from ES index
func (es *elasticsearch) getPipelines(ctx context.Context) ([]ESPipelineDoc, error) {
	return es.getPipelinesEs7(ctx)
}

// To get pipeline from ES index
func (es *elasticsearch) getPipeline(ctx context.Context, pipelineID string) (*ESPipelineDoc, error) {
	return es.getPipelineEs7(ctx, pipelineID)
}

// To get the pipelines size
func (es *elasticsearch) getPipelinesSize(ctx context.Context) (*int64, error) {
	return es.getPipelinesSizeEs7(ctx)
}
