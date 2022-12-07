package util

// Elasticsearch envs
const EsURLKey = "ES_URL"
const EsHeaderKey = "ES_HEADER"

var esURL string
var esHeader string

// ESAccess will store details about accessing ES
// that the user has provided if they chose `elasticsearch`
// as a backend.
type ESAccess struct {
	URL    string
	Header string
}

// tenantToESAccess will be a map of the raw tenantID
// to the ESAccess credentials
var tenantToESAccess map[string]ESAccess

// SetESAccessForTenant will set the ESAccess values for
// the passed `tenantId`
func SetESAccessForTenant(tenantID string, esAccess ESAccess) {
	tenantToESAccess[tenantID] = esAccess
}

// GetESAccessForTenant will get the ESAccess values for the
// passed `tenantId`.
//
// The first value returned will be the ES_URL and the second
// value will be the ES_HEADER.
func GetESAccessForTenant(tenantID string) ESAccess {
	ESAccessDetails, isExists := tenantToESAccess[tenantID]
	if !isExists {
		return ESAccess{}
	}

	return ESAccessDetails
}

func GetGlobalESURL() string {
	return esURL
}

func SetGlobalESURL(url string) {
	esURL = url
}

func GetGlobalESHeader() string {
	return esHeader
}

func SetGlobalESHeader(header string) {
	esHeader = header
}

// Opensearch envs
const OsURLKey = "OS_URL"
const OsHeaderKey = "OS_HEADER"

var osURL string
var osHeader string

func GetGlobalOSURL() string {
	return osURL
}

func SetGlobalOSURL(url string) {
	osURL = url
}

func GetGlobalOSHeader() string {
	return osHeader
}

func SetGlobalOSHeader(header string) {
	osHeader = header
}
