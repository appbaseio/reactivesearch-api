package elasticsearch

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"github.com/appbaseio/reactivesearch-api/model/reindex"
	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
)

// tenantToStorageMap will contain the storage used
// by each tenant in bytes
var tenantToStorageMap = make(map[string]int)

// FetchStorageFromES will fetch the storage details from ES
// and cache them locally
func FetchStorageFromES() error {
	indicesList := make([]reindex.AliasedIndices, 0)

	v := url.Values{}
	v.Set("format", "json")
	v.Set("bytes", "b")

	requestOptions := es7.PerformRequestOptions{
		Method: "GET",
		Path:   "/_cat/indices",
		Params: v,
	}

	// Get the client ready for the request
	//
	// If the request is for a multi-tenant setup and the backend
	// is `system`, we need to use the system client to make the call.
	esClient, clientFetchErr := util.GetESClientForTenant(context.Background())
	if clientFetchErr != nil {
		return clientFetchErr
	}

	response, err := esClient.PerformRequest(context.Background(), requestOptions)
	if err != nil {
		return err
	}

	if response.StatusCode > 300 {
		return errors.New(string(response.Body))
	}

	err = json.Unmarshal(response.Body, &indicesList)
	if err != nil {
		return err
	}

	// Reset the map when fetched
	newTenantToStorageMap := make(map[string]int)

	// Iterate over each index and extract the tenant ID to add the usage to
	for _, indexEach := range indicesList {
		_, tenantID := util.RemoveTenantID(indexEach.Index)
		if tenantID == "" {
			continue
		}

		// Get older storage used for tenant
		storageAlreadyUsed, isExists := newTenantToStorageMap[tenantID]
		if !isExists {
			storageAlreadyUsed = 0
		}

		storageUsedAsStr := indexEach.PriStoreSize
		storageUsed, convertErr := strconv.Atoi(storageUsedAsStr)
		if convertErr != nil {
			continue
		}

		storageAlreadyUsed += storageUsed
		newTenantToStorageMap[tenantID] = storageAlreadyUsed
	}

	tenantToStorageMap = newTenantToStorageMap
	return nil
}
