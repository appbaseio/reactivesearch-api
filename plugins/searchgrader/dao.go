package searchgrader

import (
	"context"
	"fmt"
	"log"

	"github.com/appbaseio-confidential/reactivesearch/util"
)

type elasticsearch struct {
	indexName string
}

func initPlugin(searchGraderIndex, mapping string) (*elasticsearch, error) {
	es := &elasticsearch{searchGraderIndex}

	ctx := context.Background()

	// Check if the searchgrader index already exists
	exists, err := util.GetClient7().IndexExists(searchGraderIndex).Do(ctx)
	if err != nil {
		return es, fmt.Errorf("error while checking if index already exists: %v", err)
	}
	if exists {
		log.Printf("%s: index named '%s' already exists, skipping...", logTag, searchGraderIndex)
		return es, nil
	}

	replicas := util.GetReplicas()
	settings := fmt.Sprintf(mapping, replicas)

	// Meta index does not exists, create a new one
	_, err = util.GetClient7().CreateIndex(searchGraderIndex).Body(settings).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while creating index named %s: %v", searchGraderIndex, err)
	}

	log.Printf("%s successfully created index named '%s'", logTag, searchGraderIndex)
	return es, nil
}

func (es *elasticsearch) updateGrade(ctx context.Context, record ESRecord) error {
	docID := generateDocID(*record.DocID, *record.Query)

	esDoc := ESDoc{
		Index: record.Index,
		Query: record.Query,
		DocID: record.DocID,
		Grade: record.Grade,
	}
	return es.updateGradeEs7(ctx, docID, esDoc)
}

func (es *elasticsearch) getMetrics(ctx context.Context, record GradeMetricsRequest) (*GradeMetricsResponse, *int, error) {
	return es.getMetricsEs7(ctx, record)
}

func (es *elasticsearch) getDocuments(ctx context.Context, query string) (map[string]interface{}, error) {
	return es.getDocumentsEs7(ctx, query)
}
