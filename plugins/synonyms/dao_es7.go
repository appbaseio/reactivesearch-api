package synonyms

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/appbaseio/reactivesearch-api/util"
	"github.com/google/uuid"
	"github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

func (es *elasticsearch) getSynonymsEs7(ctx context.Context, indexName string) ([]SynonymsStruct, error) {
	var termQuery = elastic.NewTermQuery("index.keyword", indexName)
	response, err := util.GetClient7().
		Search().Index(es.synonymsIndex).
		Query(termQuery).
		Size(1000).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving synonyms records:", err)
		return nil, err
	}

	var synonymsMap []SynonymsStruct

	for _, hit := range response.Hits.Hits {
		var synonyms SynonymsStruct
		err := json.Unmarshal(hit.Source, &synonyms)
		synonyms.Id = hit.Id
		if err != nil {
			log.Errorln(logTag, ": error while unmarshalling synonyms record:", err)
		} else {
			synonymsMap = append(synonymsMap, synonyms)
		}
	}

	return synonymsMap, nil
}

func (es *elasticsearch) putSynonymsEs7(record []SynonymsStruct, index string, ctx context.Context) (error, []SynonymsStruct) {
	bulkRequest := util.GetClient7().Bulk()
	for i, synonym := range record {
		if synonym.Index == "" {
			synonym.Index = index
		}
		if synonym.Id == "" {
			synonym.Id = uuid.New().String()
		}

		// Append index to the synonym when not present already
		if !strings.Contains(synonym.Id, index) {
			synonym.Id = AppendIndexToSynonymID(synonym.Id, index)
		}

		var tempSynonym BaseSynonymStruct
		tempSynonym.Index = synonym.Index
		tempSynonym.Type = synonym.Type
		tempSynonym.Synonym = synonym.Synonym
		record[i] = synonym
		br := elastic.NewBulkIndexRequest().Index(es.synonymsIndex).
			Id(synonym.Id).
			Doc(tempSynonym)
		bulkRequest.Add(br)
	}
	// Execute bulk request
	if len(record) != 0 {
		_, err := bulkRequest.Refresh("wait_for").Do(ctx)
		if err != nil {
			log.Errorln(logTag, ": error executing synonyms bulk request:", err)
			return err, nil
		}
	}

	return nil, record
}

func (es *elasticsearch) deleteAllSynonymsEs7(ctx context.Context, index string) error {
	deleteQuery := elastic.NewMatchQuery("index.keyword", index)
	_, err := util.GetClient7().DeleteByQuery().Index(es.synonymsIndex).Query(deleteQuery).Do(ctx)
	return err
}
