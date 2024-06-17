package storedquery

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/appbaseio-confidential/reactivesearch/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) getStoredQueriesEs7(ctx context.Context) ([]ESStoredQueryDOC, error) {
	response, err := util.GetClient7().
		Search().
		Index(es.indexName).
		Size(10000).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving stored queries record:", err)
		return nil, err
	}

	var final []ESStoredQueryDOC

	for _, hit := range response.Hits.Hits {
		var storedQuery ESStoredQueryDOC
		err := json.Unmarshal(hit.Source, &storedQuery)
		if err != nil {
			log.Errorln(logTag, ": error while unmarshalling storedQuery record:", err)
		} else {
			final = append(final, storedQuery)
		}
	}

	return final, nil
}

func (es *elasticsearch) validateQueryEs7(ctx context.Context, query string) (*bool, error) {
	opt := es7.PerformRequestOptions{
		Method:      "POST",
		Path:        "/_validate/query",
		Body:        query,
		ContentType: "application/json",
	}
	response, err := util.GetClient7().PerformRequest(ctx, opt)
	if err != nil {
		log.Errorln(logTag, ": error while validating the stored query:", err)
		return nil, err
	}
	var body ValidateResponse
	err2 := json.Unmarshal(response.Body, &body)
	if err2 != nil {
		log.Errorln(logTag, ": error un-marshalling the validate response:", err2)
		return nil, err2
	}
	if !body.Valid {
		return &body.Valid, errors.New("invalid stored query encountered")
	}
	return &body.Valid, nil
}

func (es *elasticsearch) deleteStoredQueryEs7(ctx context.Context, queryID string) error {
	_, err := util.GetClient7().
		Delete().
		Index(es.indexName).
		Id(queryID).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error deleteing stored query record:", err)
		return err
	}
	return nil
}

func (es *elasticsearch) executeQueryEs7(ctx context.Context, index string, query string) ([]byte, error) {
	opt := es7.PerformRequestOptions{
		Method:      "POST",
		Path:        "/" + index + "/_search",
		Body:        query,
		ContentType: "application/json",
	}
	response, err := util.GetClient7().PerformRequest(ctx, opt)
	if err != nil {
		log.Errorln(logTag, ": error making request to ES:", err)
		return nil, err
	}
	return response.Body, nil
}
