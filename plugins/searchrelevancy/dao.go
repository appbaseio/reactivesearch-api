package searchrelevancy

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/appbaseio/reactivesearch-api/util"
)

type elasticsearch struct {
	searchRelevancyIndex string
}

func initPlugin(searchRelevancyIndex string) (*elasticsearch, error) {
	ctx := context.Background()
	es := &elasticsearch{searchRelevancyIndex}

	// Check if the meta index already exists
	exists, err := util.GetClient7().IndexExists(searchRelevancyIndex).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Println(logTag, ": index named ", searchRelevancyIndex, " already exists, skipping...")
		return es, nil
	}

	replicas := util.GetReplicas()

	mappingData := mapping

	settings := fmt.Sprintf(indexSettingMapping, mappingData, util.HiddenIndexSettings(), replicas)

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(searchRelevancyIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", searchRelevancyIndex, err)

	}

	log.Println(logTag, ": successfully created index named", searchRelevancyIndex)
	return es, nil
}

func (es *elasticsearch) putSearchRelevancySettings(ctx context.Context, docID string, record SearchRelevancyStruct) error {
	_, err := util.GetClient7().
		Index().
		Refresh("wait_for").
		Index(es.searchRelevancyIndex).
		BodyJson(record).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error indexing searchrelevancy record for id=", docID, ":", err)
		return err
	}

	return nil
}

func (es *elasticsearch) deleteSearchRelevancySettings(ctx context.Context, docID string) error {
	_, err := util.GetClient7().
		Delete().
		Index(es.searchRelevancyIndex).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error deleting searchrelevancy record for id=", docID, ":", err)
		return err
	}

	return nil
}

func (es *elasticsearch) getSearchRelevancySettings(ctx context.Context) (map[string]SearchRelevancyStruct, error) {
	return es.getRelevancySettingsEs7(ctx)
}
