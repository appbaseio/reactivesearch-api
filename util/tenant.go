package util

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
