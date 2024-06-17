package reindex

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/appbaseio-confidential/reactivesearch/util"
	"github.com/olivere/elastic"
	log "github.com/sirupsen/logrus"
)

// updateSynonymsEs7 will update the synonyms for the new index by going through
// the older index, finding all the synonyms belonging to it and adding all of
// them for the new index as well.
func updateSynonymsEs7(ctx context.Context, sourceIndex string, destinationIndex string) error {
	// Use the source index to fetch all the synonyms that are attached to it.
	var termQuery = elastic.NewTermQuery("index.keyword", sourceIndex)
	response, err := util.GetClient7().
		Search().Index(getSynonymsIndex()).
		Query(termQuery).
		Size(1000).
		Do(ctx)

	if err != nil {
		log.Errorln(logTag, ": error retrieving synonyms records:", err)
		return err
	}

	// Return if no synonyms are found
	if len(response.Hits.Hits) == 0 {
		return nil
	}

	// Now that we have all the synonyms for the older index, we can
	// send a bulk request

	bulkRequest := util.GetClient7().Bulk()

	for _, hit := range response.Hits.Hits {
		synonymAsMap := make(map[string]interface{})
		unmarshalErr := json.Unmarshal(hit.Source, &synonymAsMap)
		if unmarshalErr != nil {
			log.Errorln(logTag, ": failed to unmarshal synonym with ID: ", hit.Id, ": with error: ", unmarshalErr.Error())
			continue
		}

		destinationID := strings.Replace(hit.Id, sourceIndex, destinationIndex, -1)
		synonymAsMap["index"] = destinationIndex

		br := elastic.NewBulkIndexRequest().Index(getSynonymsIndex()).
			Id(destinationID).
			Doc(synonymAsMap)
		bulkRequest.Add(br)
	}

	_, bulkErr := bulkRequest.Refresh("wait_for").Do(ctx)
	if bulkErr != nil {
		log.Errorln(logTag, ": error executing synonyms bulk request:", bulkErr.Error())
		return bulkErr
	}

	return nil

}
