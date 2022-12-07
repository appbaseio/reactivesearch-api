package elasticsearch

import (
	"context"
	"fmt"
	"os"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
)

// initSystemESClient will initiate the system ES client
// that will be used to make all calls to the system
// ES index.
//
// The system ES index is the one that will be used for
// all users whose backend is set to `system`
//
// We only want this client if Arc is being run in
// multi-tenant SLS
func initSystemESClient() (*es7.Client, error) {
	if util.IsSLSDisabled() || !util.MultiTenant {
		return nil, nil
	}

	// Get the system ES URL
	systemESUrl := os.Getenv(systemESUrlKey)
	if systemESUrl == "" {
		// Throw an error
		return nil, fmt.Errorf("`%s` not present in environment!", systemESUrlKey)
	}

	loggerT := log.New()
	wrappedLoggerDebug := &util.WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &util.WrapKitLoggerError{*loggerT}

	esHttpClient := util.HTTPClient()

	client7, err := es7.NewClient(
		es7.SetURL(systemESUrl),
		es7.SetRetrier(util.NewRetrier()),
		es7.SetSniff(util.IsSniffingEnabled()),
		es7.SetHttpClient(esHttpClient),
		es7.SetErrorLog(wrappedLoggerError),
		es7.SetInfoLog(wrappedLoggerDebug),
		es7.SetTraceLog(wrappedLoggerDebug),
	)
	if err != nil {
		log.Fatal("Error encountered: ", fmt.Errorf("error while initializing elastic v7 client: %v", err))
	}

	return client7, nil
}

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
