package elasticsearch

import (
	"context"
	"fmt"

	"github.com/appbaseio/reactivesearch-api/model/domain"
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

	loggerT := log.New()
	wrappedLoggerDebug := &util.WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &util.WrapKitLoggerError{*loggerT}

	esHttpClient := util.HTTPClient()

	client7, err := es7.NewClient(
		es7.SetURL(util.GetSystemESURL()),
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

// GetESClientForTenant will get the esClient for the tenant so that
// it can be used to make requests
func (es *elasticsearch) GetESClientForTenant(ctx context.Context) (*es7.Client, error) {
	if util.IsSLSDisabled() || !util.MultiTenant {
		return util.GetClient7(), nil
	}

	// Check the backend and accordingly determine the client.
	domain, domainFetchErr := domain.FromContext(ctx)
	if domainFetchErr != nil {
		errMsg := fmt.Sprintf("error while fetching domain info from context: %s", domainFetchErr.Error())
		return nil, fmt.Errorf(errMsg)
	}

	backend := util.GetBackendByDomain(domain.Raw)
	if *backend == util.System {
		return es.systemESClient, nil
	}

	// If backend is not `system`, this route can be called for an ES
	// backend only.
	//
	// We will have to fetch the ES_URL value from global vars and create
	// a simple client using that.

	// Fetch the tenantId using the domain
	tenantId := util.GetTenantForDomain(domain.Raw)
	esAccess := util.GetESAccessForTenant(tenantId)

	if esAccess.URL == "" {
		errMsg := "ES_URL not defined in global vars, cannot continue without that!"
		return nil, fmt.Errorf(errMsg)
	}

	// NOTE: We are assuming that basic auth will be provided in the URL itself.
	//
	// We will deprecate support for ES_HEADER since pipelines can work without
	// the header as well by accepting basic auth in the URL itself.
	esURLParsed, parseErr := util.ParseESURL(esAccess.URL, esAccess.Header)
	if parseErr != nil {
		errMsg := fmt.Sprint("Error while parsing ES_URL and ES_HEADER: ", parseErr.Error())
		return nil, fmt.Errorf(errMsg)
	}

	loggerT := log.New()
	wrappedLoggerDebug := &util.WrapKitLoggerDebug{*loggerT}
	wrappedLoggerError := &util.WrapKitLoggerError{*loggerT}

	esClient, clientErr := es7.NewSimpleClient(
		es7.SetURL(esURLParsed),
		es7.SetRetrier(util.NewRetrier()),
		es7.SetHttpClient(util.HTTPClient()),
		es7.SetErrorLog(wrappedLoggerError),
		es7.SetInfoLog(wrappedLoggerDebug),
		es7.SetTraceLog(wrappedLoggerDebug),
	)

	if clientErr != nil {
		errMsg := fmt.Sprint("error while initiating client to make request: ", clientErr.Error())
		return nil, fmt.Errorf(errMsg)
	}

	return esClient, nil
}
