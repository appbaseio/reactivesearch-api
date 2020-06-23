package response

import "sync"

// CurrentResponseProcessMutex to stop concurrent writes on map
var CurrentResponseProcessMutex = sync.Mutex{}

// Response represents the cached API response for a request
// Key is the unique ID for each request
var Response = make(map[string]map[string]interface{})

// GetResponse returns the response by request ID
func GetResponse(requestID string) map[string]interface{} {
	CurrentResponseProcessMutex.Lock()
	defer CurrentResponseProcessMutex.Unlock()
	response, ok := Response[requestID]
	if !ok {
		return nil
	}
	return response
}

// SaveResponse returns the response by request ID
func SaveResponse(requestID string, response map[string]interface{}) {
	CurrentResponseProcessMutex.Lock()
	defer CurrentResponseProcessMutex.Unlock()
	Response[requestID] = response
}

// ClearResponse clears the cache for a particular request ID
func ClearResponse(requestID string) {
	CurrentResponseProcessMutex.Lock()
	defer CurrentResponseProcessMutex.Unlock()
	delete(Response, requestID)
}
