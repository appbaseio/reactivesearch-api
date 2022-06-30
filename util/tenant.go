package util

import (
	"context"

	es7 "github.com/olivere/elastic/v7"
)

// GetTenantID will return the tenant ID that
// can be used in various places.
//
// ArcID will be used as tenant_id but this method
// will take care of handling errors.
// This is just a wrapper over GetArcID()
//
// This function is just added to keep the notion of
// tenantID alive and in case the arcID and tenant ID
// become separate entities in the future.
func GetTenantID() (string, error) {
	tenantId, tenantIdErr := GetArcID()
	return tenantId, tenantIdErr
}

// IndexRequestDo will handle index request to be made
// to ES through Olivere/Elastcisearch and the request and response modifications.
//
// This method will modify the ES request to add tenant_id
// before sending the request to all documents.
//
// This method should be called whenever an index request is made
// to ES through olivere/elasticsearch library.
func IndexRequestDo(requestAsService *es7.IndexService, ctx context.Context) (*es7.IndexResponse, error) {
	// There is no need to add tenant ID if the request is being
	// made to an external ES so we can just do a normal
	// index Do and return the response.
	if ExternalElasticsearch == "true" {
		return requestAsService.Do(ctx)
	}
}
