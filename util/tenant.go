package util

import (
	"fmt"
	"strings"
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
