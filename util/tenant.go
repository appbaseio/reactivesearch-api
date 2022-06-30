package util

import (
	"context"
	"errors"
	"fmt"

	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
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
func IndexRequestDo(requestAsService *es7.IndexService, originalBody interface{}, ctx context.Context) (*es7.IndexResponse, error) {
	// There is no need to add tenant ID if the request is being
	// made to an external ES so we can just do a normal
	// index Do and return the response.
	if ExternalElasticsearch == "true" {
		return requestAsService.BodyJson(originalBody).Do(ctx)
	}

	// Modify the originalBody and add it before sending
	// the request.
	//
	// NOTE: Assumption is that the body will be a map of string to
	// interface and a new string will be added `tenant_id`
	bodyAsMap, asMapOk := originalBody.(map[string]interface{})

	if !asMapOk {
		errMsg := "error while converting original request body to add `tenant_id`"
		log.Warnln(": ", errMsg)
		return nil, errors.New(errMsg)
	}

	tenantId, tenantIdErr := GetTenantID()
	if tenantIdErr != nil {
		errToReturn := fmt.Errorf("error while getting tenant ID: %s", tenantIdErr)
		log.Warnln(": ", errToReturn.Error())
		return nil, errToReturn
	}

	bodyAsMap["tenant_id"] = tenantId

	// Finally make the request
	esResponse, esResponseErr := requestAsService.BodyJson(bodyAsMap).Do(ctx)

	// Index response doesn't return the source body so the response
	// can be returned directly without need for checking
	// or modifying anything.
	return esResponse, esResponseErr
}
