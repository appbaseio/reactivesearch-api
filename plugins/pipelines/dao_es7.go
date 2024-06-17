package pipelines

import (
	"context"
	"encoding/json"

	"github.com/appbaseio-confidential/reactivesearch/util"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) createPipelineEs7(ctx context.Context, pipelineID string, record ESPipelineDoc) error {
	_, err := util.GetClient7().
		Index().
		Index(es.indexName).
		BodyJson(record).
		Refresh("wait_for").
		Id(pipelineID).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error indexing pipeline record:", err)
		return err
	}
	return nil
}

func (es *elasticsearch) updatePipelineEs7(ctx context.Context, pipelineID string, record ESPipelineDoc) error {
	_, err := util.GetClient7().
		Update().
		Index(es.indexName).
		Id(pipelineID).
		Doc(record).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error updating pipeline record:", err)
		return err
	}
	return nil
}

func (es *elasticsearch) deletePipelineEs7(ctx context.Context, pipelineID string) error {
	_, err := util.GetClient7().
		Delete().
		Index(es.indexName).
		Id(pipelineID).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error deleteing pipeline record:", err)
		return err
	}
	return nil
}

func (es *elasticsearch) getPipelinesEs7(ctx context.Context) ([]ESPipelineDoc, error) {
	response, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Size(int(getSizeLimitByPlan())).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving pipelines record:", err)
		return nil, err
	}

	var final []ESPipelineDoc

	// NOTE: The following should not happen anymore with the es7 client either.
	// On version 8, then the response object will not have a valid Hits value
	// so it will end up with an out of memory error.
	if response.Hits == nil || response.Hits.Hits == nil {
		return final, nil
	}

	for _, hit := range response.Hits.Hits {
		var pipeline ESPipelineDoc
		err := json.Unmarshal(hit.Source, &pipeline)
		if err != nil {
			log.Errorln(logTag, ": error while unmarshalling pipeline record:", err)
		} else {
			final = append(final, pipeline)
		}
	}

	return final, nil
}

func (es *elasticsearch) getPipelinesSizeEs7(ctx context.Context) (*int64, error) {
	response, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Size(0).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving pipelines size:", err)
		return nil, err
	}
	return &response.Hits.TotalHits.Value, nil
}

func (es *elasticsearch) getPipelineEs7(ctx context.Context, pipelineID string) (*ESPipelineDoc, error) {
	response, err := util.GetClient7().
		Get().
		Id(pipelineID).
		Index(es.indexName).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving pipeline record:", err)
		return nil, err
	}

	var final ESPipelineDoc

	var pipeline ESPipelineDoc
	err2 := json.Unmarshal(response.Source, &pipeline)
	if err2 != nil {
		log.Errorln(logTag, ": error while unmarshalling pipeline record:", err)
	} else {
		final = pipeline
	}

	return &final, nil
}
