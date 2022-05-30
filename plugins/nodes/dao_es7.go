package nodes

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
)

// pingES7 will ping ElasticSearch based on the passed machine ID
// with the current unix timestamp.
//
// This function will also determine whether the document should
// be created or updated based on the machineID being present
// or not being present in ES.
func (es *elasticsearch) pingES7(ctx context.Context, machineID string) error {
	// Check if the ID already exists
	idExists, err := es.machineExists(ctx, machineID)
	if err != nil {
		return err
	}

	if idExists {
		// Update the ping time
	} else {
		// Create a new doc with ping time
	}

	return nil
}

// machineExists will check if the machineID exists in the index
// using the exists query provided by elasticsearch
func (es *elasticsearch) machineExists(ctx context.Context, machineID string) (bool, error) {
	response, err := util.GetClient7().Get().Index(es.indexName).Id(machineID).Do(ctx)

	if err != nil {
		return false, err
	}

	return response != nil, nil
}
