package pipelines

import (
	"encoding/json"
	"sort"
)

// cachedPipelines represents the struct of a list of saved pipelines in the .pipelines index
var cachedPipelines []ESPipelineDoc

// SetPipelinesToCache sets the pipelines into cache for faster access
func SetPipelinesToCache(pipelines []ESPipelineDoc) {
	pipelinesToCache := []ESPipelineDoc{}

	// Decode the script strings
	for _, pipeline := range pipelines {
		pipelinesToCache = append(pipelinesToCache, addInitVersionIfNotPresent(decodePipelineScripts(pipeline)))
	}

	cachedPipelines = pipelinesToCache
}

// Add the passed pipeline to cache
func AddPipelineToCache(pipeline ESPipelineDoc) {
	// Decode the pipeline scripts and then save it in cache.
	cachedPipelines = append(cachedPipelines, decodePipelineScripts(pipeline))
}

// Check if the pipeline is present in the cache
func GetPipelineAndLocFromCache(pipelineID string) (*ESPipelineDoc, *int) {
	for location, pipeline := range cachedPipelines {
		if *pipeline.ID == pipelineID {
			return &pipeline, &location
		}
	}
	return nil, nil
}

// Update the pipeline in the cache using the ID
// Since the file will be verified by parent, this method will
// do a direct cache update for the passed ID without any validations
// for nil etc.
func UpdatePipelineInCache(pipelineID string, pipelineBody ESPipelineDoc) bool {
	_, location := GetPipelineAndLocFromCache(pipelineID)
	cachedPipelines[*location] = decodePipelineScripts(pipelineBody)

	return true
}

// Delete the pipeline from the cache
func DeletePipelineFromCache(pipelineID string) bool {
	_, location := GetPipelineAndLocFromCache(pipelineID)
	if location == nil {
		return false
	}

	cachedPipelines = append(cachedPipelines[:*location], cachedPipelines[*location+1:]...)
	return true
}

// Get all pipelines from cache
// The array will be sorted based on decreasing
// priority
func GetPipelinesFromCache() []ESPipelineDoc {
	sort.Slice(cachedPipelines, func(i, j int) bool {
		cachedPipelineI := GetLivePipeline(cachedPipelines[i])
		cachedPipelineJ := GetLivePipeline(cachedPipelines[j])
		if cachedPipelineI.Priority != nil && cachedPipelineJ.Priority != nil {
			return *cachedPipelineI.Priority > *cachedPipelineJ.Priority
		}
		return false
	})
	return cachedPipelines
}

// GetPipelineVersion will get the pipeline version from the pipelines
// versions. If not present, it will return nil which would indicate
// not found.
func GetPipelineVersion(pipeline ESPipelineDoc, version int) (*Version, *int) {
	if *pipeline.Versions == nil {
		return nil, nil
	}
	for versionIndex, versionOfPipeline := range *pipeline.Versions {
		if *versionOfPipeline.Version == version {
			return &versionOfPipeline, &versionIndex
		}
	}

	return nil, nil
}

// GetLivePipeline will get the live pipeline for the passed
// pipeline.
//
// If the live versions are empty or not set the default pipelines
// will be returned.
func GetLivePipeline(pipeline ESPipelineDoc) ESPipelineDoc {
	if pipeline.Versions == nil || len(*pipeline.Versions) == 0 || pipeline.LiveVersion == nil {
		return pipeline
	}

	// Version is present so extract it and return
	// Get the live version from the cache
	pipelineLiveVersion, _ := GetPipelineVersion(pipeline, *pipeline.LiveVersion)
	if pipelineLiveVersion != nil {
		// Unmarshal the pipeline into a ESPipelineDoc container
		// Unmarshal the current version of the pipeline
		var currentVersionPipeline ESPipelineDoc
		json.Unmarshal([]byte(*pipelineLiveVersion.Content), &currentVersionPipeline)

		return currentVersionPipeline
	}

	return pipeline
}
