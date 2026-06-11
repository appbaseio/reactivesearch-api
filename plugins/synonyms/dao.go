package synonyms

import (
	"context"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/util"
	log "github.com/sirupsen/logrus"
)

type elasticsearch struct {
	synonymsIndex string
}

func initPlugin(synonymsIndex, mapping string) (*elasticsearch, error) {
	ctx := context.Background()

	es := &elasticsearch{synonymsIndex}

	// Check if the meta index already exists
	exists, err := util.GetClient7().IndexExists(synonymsIndex).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Println(logTag, ": index named ", synonymsIndex, " already exists, skipping...")
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := util.AdaptIndexBody(fmt.Sprintf(mapping, replicas))

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(synonymsIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", synonymsIndex, err)
	}

	log.Println(logTag, ": successfully created index named", synonymsIndex)
	return es, nil
}

func (es *elasticsearch) putSynonyms(ctx context.Context, record []SynonymsStruct, index string) (error, []SynonymsStruct) {
	return es.putSynonymsEs7(record, index, ctx)
}

func (es *elasticsearch) deleteSynonyms(ctx context.Context, docID string) error {
	_, err := util.GetClient7().
		Delete().Refresh("wait_for").
		Index(es.synonymsIndex).
		Id(docID).
		Do(ctx)
	if err != nil {
		log.Errorln(logTag, ": error deleting synonyms record for id=", docID, ":", err)
		return err
	}

	return nil
}

func (es *elasticsearch) getSynonyms(ctx context.Context, indexName string) ([]SynonymsStruct, error) {
	return es.getSynonymsEs7(ctx, indexName)
}

func (es *elasticsearch) deleteAllSynonyms(ctx context.Context, index string) error {
	return es.deleteAllSynonymsEs7(ctx, index)
}
