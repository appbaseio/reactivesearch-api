package response

import "sync"

// CurrentResponseProcessMutex to stop concurrent writes on map
var CurrentResponseProcessMutex = sync.RWMutex{}

// Response represents the cached API response for a request
// Key is the unique ID for each request
var Response = sync.Map{}

// GetResponse returns the response by request ID
func GetResponse(requestID string) map[string]interface{} {
	response, ok := Response.Load(requestID)
	if !ok {
		return nil
	}
	responseAsMap, ok := response.(map[string]interface{})
	if !ok {
		return nil
	}
	return responseAsMap
}

// SaveResponse returns the response by request ID
func SaveResponse(requestID string, response map[string]interface{}) {
	Response.Store(requestID, response)
}

// ClearResponse clears the cache for a particular request ID
func ClearResponse(requestID string) {
	Response.Delete(requestID)
}
