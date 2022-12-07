package elasticsearch

// Store the indices for tenants using a tenant to index map
//
// The name of the index will be stored without the `tenant_id`
// appended in it.
var tenantToIndexMap map[string][]string

// SetIndexesToCache will set the index into the cache map
func SetIndexesToCache(tenantID string, index string) {

}
