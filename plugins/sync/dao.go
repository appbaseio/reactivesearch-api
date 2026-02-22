package sync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(rulesIndex, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{rulesIndex}

	ctx := context.Background()

	// Check if the rules index already exists
	exists, err := util.GetClient7().IndexExists(rulesIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, rulesIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, util.HiddenIndexSettings(), replicas)

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(rulesIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", rulesIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, rulesIndex)
	return es, nil
}

func (es *elasticsearch) saveSyncPreferences(ctx context.Context, record SyncPreferences) (*es7.IndexResponse, error) {
	return util.GetClient7().Index().
		Refresh("wait_for").
		Index(es.indexName).
		Id(syncPreferenceDocID).
		BodyJson(record).
		Do(ctx)
}

func (es *elasticsearch) getSyncPreferences(ctx context.Context) (SyncPreferences, error) {
	var record SyncPreferences
	response, err := util.GetClient7().Get().
		Index(es.indexName).
		Id(syncPreferenceDocID).
		Do(ctx)
	if err != nil {
		if es7.IsNotFound(err) {
			return SyncPreferences{}, nil
		}
		log.Errorln(logTag, ": error retriving sync preferences", err)
		return record, err
	}
	err = json.Unmarshal(response.Source, &record)
	if err != nil {
		log.Errorln(logTag, ": error un-marshalling sync preferences", err)
		return record, err
	}
	return record, nil
}
