package util

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/appbaseio/reactivesearch-api/model/domain"
)

const (
	tenantIdSeparator = "_$_tenant_$_"
)

// AppendTenantID will append the tenant ID to the passed string if it
// is not present
func AppendTenantID(appendTo string, tenantId string) string {
	if !strings.Contains(appendTo, fmt.Sprintf("%s%s", tenantIdSeparator, tenantId)) {
		return fmt.Sprintf("%s%s%s", appendTo, tenantIdSeparator, tenantId)
	}
	return appendTo
}

// RemoveTenantID will remove the tenantID from the string and return
// both the original string and tenantId separately
func RemoveTenantID(removeFrom string) (string, string) {
	if !strings.Contains(removeFrom, tenantIdSeparator) {
		return removeFrom, ""
	}

	// Split the string using the separator
	splittedRemoveFrom := strings.Split(removeFrom, tenantIdSeparator)
	if len(splittedRemoveFrom) < 2 {
		return splittedRemoveFrom[0], ""
	}

	return splittedRemoveFrom[0], splittedRemoveFrom[1]
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
