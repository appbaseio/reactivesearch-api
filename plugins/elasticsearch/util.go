package elasticsearch

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
)

// CacheIndexesForTenants will fetch all the indexes from the system
// ES and then filter them into different tenants and accordingly
// cache them into the cache map
//
// This function will only execute if SLS is enabled and Multi-Tenant
// is enabled
func CacheIndexesForTenants(systemESClient *es7.Client, ctx context.Context) error {
	if util.IsSLSDisabled() || !util.MultiTenant {
		return nil
	}

	// Make a _cat/indices call to get all the indexes for the tenant
	indices, indicesFetchErr := systemESClient.CatIndices().Do(ctx)
	if indicesFetchErr != nil {
		return indicesFetchErr
	}

	for _, index := range indices {
		// Use the name of the index to extract the tenant_id and then
		// cache it accordingly.
		strippedIndexName, tenantId := util.RemoveTenantID(index.Index)

		// Not likely, but there can be indexes that do not have the
		// tenantId appended to the name of the index. In such a case,
		// we can skip these indexes
		if tenantId == "" {
			continue
		}

		SetIndexToCache(tenantId, strippedIndexName)
	}

	return nil
}
