package reindex

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func updateSynonymsEs7(ctx context.Context, script string, index string, params map[string]interface{}) error {
	// Get the client ready for the request
	//
	// If the request is for a multi-tenant setup and the backend
	// is `system`, we need to use the system client to make the call.
	esClient, clientFetchErr := util.GetESClientForTenant(ctx)
	if clientFetchErr != nil {
		log.Warnln(logTag, ": ", clientFetchErr)
		return clientFetchErr
	}

	query := es7.NewTermQuery("index.keyword", index)
	_, err := esClient.
		UpdateByQuery().
		Query(query).
		Index(getSynonymsIndex()).
		Script(es7.NewScript(script).Params(params)).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error updating synonyms for index=", index, ":", err)
		return err
	}
	return nil
}
