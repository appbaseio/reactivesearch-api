package util

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// AppendTenantID will append the `tenant_id` to the string
// passed.
func AppendTenantID(appendTo string) (string, error) {
	tenantId, tenantIdErr := GetArcID()

	if tenantIdErr != nil {
		return appendTo, tenantIdErr
	}

	return fmt.Sprintf("%s_%s", appendTo, tenantId), nil
}

// RemoveTenantID will remove the `tenant_id` from the string
// passed.
func RemoveTenantID(removeFrom string) (string, error) {
	tenantId, tenantIdErr := GetArcID()

	if tenantIdErr != nil {
		return removeFrom, tenantIdErr
	}

	return strings.Replace(removeFrom, fmt.Sprint("_", tenantId), "", -1), nil
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

	bodyWithTenant, bodyWithTenantErr := addTenantId(originalBody)
	if bodyWithTenantErr != nil {
		return nil, bodyWithTenantErr
	}

	// Finally make the request
	esResponse, esResponseErr := requestAsService.BodyJson(bodyWithTenant).Do(ctx)

	// Index response doesn't return the source body so the response
	// can be returned directly without need for checking
	// or modifying anything.
	return esResponse, esResponseErr
}

// SearchRequestDo will handle search request to be made
// to ES through Olivere/Elastcisearch and the request and response modifications.
//
// This method will modify the ES request to add tenant_id
// before sending the request to all documents as well as remove
// the tenant_id field before returning the response
//
// This method should be called whenever a search request is made
// to ES through olivere/elasticsearch library.
func SearchRequestDo(requestAsService *es7.SearchService, searchQuery es7.Query, ctx context.Context) (*es7.SearchResult, error) {
	// There is no need to add tenant ID if the request is being
	// made to an external ES so we can just do a normal
	// search Do and return the response.
	if ExternalElasticsearch == "true" {
		return requestAsService.Do(ctx)
	}

	tenantId, tenantIdErr := GetTenantID()
	if tenantIdErr != nil {
		errToReturn := fmt.Errorf("error while getting tenant ID: %s", tenantIdErr)
		log.Warnln(": ", errToReturn.Error())
		return nil, errToReturn
	}

	termQueryTenantId := es7.NewTermQuery("tenant_id", tenantId)

	queriesToPass := []es7.Query{termQueryTenantId}
	if searchQuery != nil {
		queriesToPass = append(queriesToPass, searchQuery)
	}

	tenantIdFilterQuery := es7.NewBoolQuery().Filter(queriesToPass...)

	esResponse, esResponseErr := requestAsService.Query(tenantIdFilterQuery).Do(ctx)

	if esResponseErr != nil {
		return esResponse, esResponseErr
	}

	// Modify the response before returning it and remove
	// `tenant_id` field from all docs.
	for hitIndex, hit := range esResponse.Hits.Hits {
		// Extract the source, remove the `tenant_id` and replace it.
		originalSource := make(map[string]interface{})
		originalSrcErr := json.Unmarshal(hit.Source, &originalSource)

		if originalSrcErr != nil {
			errMsg := fmt.Errorf("error while unmarshalling original source for index %d with error: %s", hitIndex, originalSrcErr)
			log.Warnln(": ", errMsg.Error())
			return esResponse, errMsg
		}

		// Once the unmarshal is done, remove the `tenant_id` key from the
		// JSON.
		delete(originalSource, "tenant_id")

		// Marshal the updated map back to bytes
		updatedSource, marshalErr := json.Marshal(originalSource)
		if marshalErr != nil {
			errToReturn := fmt.Errorf("error while marshalling updated source back to bytes: %s", marshalErr)
			log.Warnln(": ", errToReturn.Error())
			return esResponse, errToReturn
		}

		// Update the hit source.
		esResponse.Hits.Hits[hitIndex].Source = updatedSource
	}

	// Finally return the response
	return esResponse, esResponseErr
}

// DeleteRequestDo will handle delete requests to be made to
// ES through olivere/elasticsearch and the request and response
// modification if any is required.
//
// Only one of `DeleteService` or `DeleteByQueryService` is accepted
// as input.
//
// This method will modify the DeleteService to DeleteByQueryService
// and add the `tenant_id` as a filter.
// The above will only be done if the ExternalElasticsearch flag is not
// `true`.
//
// The response is an interface and can be converted to one of following:
// - DeleteService -> DeleteResponse
// - DeleteByQueryService -> BulkIndexByScrollResponse
//
// This method should be called whenever a delete request is
// made to ES and tenant related methods were used to index.
func DeleteRequestDo(requestAsService interface{}, ctx context.Context, id interface{}, index string) (interface{}, error) {

	// Define a variable of DeleteByQueryService to use finally
	var deleteByQuery *es7.DeleteByQueryService

	// The `requestAsService` will only support either
	// DeleteByQueryService or DeleteService.
	switch requestType := requestAsService.(type) {
	case *es7.DeleteByQueryService:
		requestAsType := requestAsService.(*es7.DeleteByQueryService)
		if ExternalElasticsearch == "true" {
			return requestAsType.Do(ctx)
		}

		// Else pass it as is to deleteByQuery
		deleteByQuery = requestAsType
	case *es7.DeleteService:
		requestAsType := requestAsService.(*es7.DeleteService)
		if ExternalElasticsearch == "true" {
			return requestAsType.Do(ctx)
		}

		// If type is deleteService then id and index should not be
		// nil or empty.
		if id == nil || index == "" {
			return nil, fmt.Errorf("`id` and `index` are required parameters for DeleteByService type")
		}

		// Else convert it to DeleteByQueryService
		// Create a filter query based on the ID
		filterByIdQuery := es7.NewMatchQuery("_id", id)

		deleteByQuery = GetClient7().DeleteByQuery().Index(index).Query(filterByIdQuery)
	default:
		return nil, fmt.Errorf("invalid type passed for DeleteRequestDo: %v", requestType)
	}

	// Get the tenant ID
	tenantId, tenantIdErr := GetTenantID()
	if tenantIdErr != nil {
		errToReturn := fmt.Errorf("error while getting tenant ID: %s", tenantIdErr)
		log.Warnln(": ", errToReturn.Error())
		return nil, errToReturn
	}

	// Finally add the `tenant_id` filter to the delete query
	tenantIdFilter := es7.NewMatchQuery("tenant_id", tenantId)

	deleteByQuery = deleteByQuery.Query(tenantIdFilter)

	return deleteByQuery.Do(ctx)
}

// UpdateRequestDo will handle update request to be made
// to ES through Olivere/Elastcisearch and the request and response modifications.
//
// This method will modify the ES request to add tenant_id
// before sending the request to all documents.
//
// This method should be called whenever an update request is made
// to ES through olivere/elasticsearch library.
func UpdateRequestDo(requestAsService *es7.UpdateService, updateBody interface{}, ctx context.Context) (*es7.UpdateResponse, error) {
	// There is no need to add tenant ID if the request is being
	// made to an external ES so we can just do a normal
	// update Do and return the response.
	if ExternalElasticsearch == "true" {
		return requestAsService.Doc(updateBody).Do(ctx)
	}

	// We need to add the tenant_id if not an external search.
	// Get the tenant ID
	bodyWithTenant, bodyWithTenantErr := addTenantId(updateBody)
	if bodyWithTenantErr != nil {
		return nil, bodyWithTenantErr
	}

	return requestAsService.Doc(bodyWithTenant).Do(ctx)
}

// CountRequestDo will handle count requests to be made to ES
// through olivere/elasticsearch and the request and response
// modification.
//
// For count request, a match query will be added that will
// contain the tenant_id so that results only for the particular
// tenant are returned.
//
// This method should be called whenever a CountService operation
// is to be done in a tenant_id present environment.
func CountRequestDo(requestAsService *es7.CountService, ctx context.Context) (int64, error) {
	// If ExternalElasticSearch is being done, just `Do` as is.
	if ExternalElasticsearch == "true" {
		return requestAsService.Do(ctx)
	}

	// Else add the `tenant_id` as a match query.
	tenantId, tenantIdErr := GetTenantID()
	if tenantIdErr != nil {
		errToReturn := fmt.Errorf("error while getting tenant ID: %s", tenantIdErr)
		log.Warnln(": ", errToReturn.Error())
		return 0, errToReturn
	}

	termQueryTenantId := es7.NewTermQuery("tenant_id", tenantId)
	tenantIdFilterQuery := es7.NewBoolQuery().Filter(termQueryTenantId)

	return requestAsService.Query(tenantIdFilterQuery).Do(ctx)
}

// addTenantId will add the tenant_id field to the passed doc.
// The doc is expected to be of type map though it will be passed
// as an interface.
//
// An error will be raised if parsing to map fails or any other
// part of adding the tenant fails.
func addTenantId(originalBody interface{}) (interface{}, error) {
	// Modify the originalBody
	//
	// NOTE: Assumption is that the body will be a map of string to
	// interface and a new string will be added `tenant_id`

	marshalledBody, marshalErr := json.Marshal(originalBody)
	if marshalErr != nil {
		errMsg := fmt.Sprint("error while unmarshalling original request body to add `tenant_id`: ", marshalErr)
		log.Warnln(": ", errMsg)
		return nil, errors.New(errMsg)
	}

	bodyAsMap := make(map[string]interface{})

	asMapErr := json.Unmarshal(marshalledBody, &bodyAsMap)

	if asMapErr != nil {
		errMsg := fmt.Sprint("error while converting original request body to add `tenant_id`: ", asMapErr)
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

	return bodyAsMap, nil
}
