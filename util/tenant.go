package util

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/appbaseio/reactivesearch-api/model/domain"
)

const (
	tenantIdSeparator     = "_$_tenant_$_"
	tenantIdSeparatorZinc = "__tenant__"
	tenantIdReplacer      = "XXXXXX"
)

// InternalAppendTenantID will append the tenant ID to the passed string if it
// is not present
func InternalAppendTenantID(appendTo string, tenantId string, separator string) string {
	if !strings.Contains(appendTo, fmt.Sprintf("%s%s", separator, tenantId)) {
		return fmt.Sprintf("%s%s%s", appendTo, separator, tenantId)
	}
	return appendTo
}

// InternalRemoveTenantID will remove the tenantID from the string and return
// both the original string and tenantId separately
func InternalRemoveTenantID(removeFrom string, separator string) (string, string) {
	if !strings.Contains(removeFrom, separator) {
		return removeFrom, ""
	}

	// Split the string using the separator
	splittedRemoveFrom := strings.Split(removeFrom, separator)
	if len(splittedRemoveFrom) < 2 {
		return splittedRemoveFrom[0], ""
	}

	return splittedRemoveFrom[0], splittedRemoveFrom[1]
}

// AppendTenantID will append the tenantID to the passed string if it is not
// present
func AppendTenantID(appendTo string, tenantId string) string {
	return InternalAppendTenantID(appendTo, tenantId, tenantIdSeparator)
}

// AppendTenantIDZinc will append the tenantID to the passed string using the
// syntax supported by Zinc
func AppendTenantIDZinc(appendTo string, tenantId string) string {
	return InternalAppendTenantID(appendTo, tenantId, tenantIdSeparatorZinc)
}

// RemoveTenantID will remove the tenantID from the passed string and return the
// raw string and the tenant ID
func RemoveTenantID(removeFrom string) (string, string) {
	return InternalRemoveTenantID(removeFrom, tenantIdSeparator)
}

// RemoveTenantIDZinc will remove the tenantID from the passed string using the
// zinc tenant ID separator and return the raw string and the tenant ID
func RemoveTenantIDZinc(removeFrom string) (string, string) {
	return InternalRemoveTenantID(removeFrom, tenantIdSeparatorZinc)
}

// AddTenantID will add the tenant ID to the passed body.
//
// The body should be a map where a top level key `tenant_id` will be
// added
func AddTenantID(bodyInBytes []byte, ctx context.Context) ([]byte, error) {
	// Fetch the domain from the context and get the tenant ID using that.
	domainFromCtx, domainFetchErr := domain.FromContext(ctx)
	if domainFetchErr != nil {
		return nil, domainFetchErr
	}

	tenantID := GetTenantForDomain(domainFromCtx.Raw)

	// Unmarshal the body into a map
	bodyAsMap := make(map[string]interface{})
	unmarshalErr := json.Unmarshal(bodyInBytes, &bodyAsMap)
	if unmarshalErr != nil {
		return nil, unmarshalErr
	}

	bodyAsMap["tenant_id"] = tenantID

	return json.Marshal(bodyAsMap)
}

// HideTenantID will replace all occurrences of the `tenant_id` with a special
// string.
func HideTenantID(bodyInBytes []byte, ctx context.Context) ([]byte, error) {
	// Fetch the domain from the context and get the tenant ID using that.
	domainFromCtx, domainFetchErr := domain.FromContext(ctx)
	if domainFetchErr != nil {
		return nil, domainFetchErr
	}

	tenantID := GetTenantForDomain(domainFromCtx.Raw)

	updatedBody := strings.Replace(string(bodyInBytes), tenantID, tenantIdReplacer, -1)
	return []byte(updatedBody), nil
}

// addTenantIdFilterQuery adds a term query to filter documents by tenant Id
func addTenantIdFilterQuery(rawQuery []byte, ctx context.Context) ([]byte, error) {
	// Fetch the domain from the context and get the tenant ID using that.
	domainFromCtx, domainFetchErr := domain.FromContext(ctx)
	if domainFetchErr != nil {
		return nil, domainFetchErr
	}

	tenantId := GetTenantForDomain(domainFromCtx.Raw)

	if tenantId != "*" {
		termQueryTenantId := map[string]interface{}{
			"term": map[string]interface{}{
				"tenant_id.keyword": tenantId,
			},
		}
		// if request body is not empty then modify the request query
		if len(rawQuery) > 0 {
			var queryJSON map[string]interface{}
			err := json.Unmarshal(rawQuery, &queryJSON)
			if err != nil {
				return nil, err
			}

			queryValue := queryJSON["query"]
			if queryValue == nil {
				queryValue = map[string]interface{}{
					"match_all": map[string]interface{}{},
				}
			}
			// check if query if filtering by `_id`
			queryMap, ok := queryValue.(map[string]interface{})
			if ok {
				termMap, ok := queryMap["term"].(map[string]interface{})
				if ok {
					idString, ok := termMap["_id"].(string)
					if ok {
						termMap["_id"] = AppendTenantID(idString, tenantId)
						queryMap["term"] = termMap
						queryValue = queryMap
					}
				}
			}
			tenantIdFilterQuery := map[string]interface{}{
				"bool": map[string]interface{}{
					"must": []interface{}{
						queryValue,
						termQueryTenantId,
					},
				},
			}
			queryJSON["query"] = tenantIdFilterQuery
			return json.Marshal(queryJSON)
		}
		return json.Marshal(map[string]interface{}{"query": termQueryTenantId})
	}
	return rawQuery, nil
}