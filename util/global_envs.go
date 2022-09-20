package util

// Elasticsearch envs
const EsURLKey = "ES_URL"
const EsHeaderKey = "ES_HEADER"

var esURL string
var esHeader string

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
