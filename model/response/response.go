package response

import "sync"

// Response represents the cached API response for a request
// Key is the unique ID for each request
var Response = sync.Map{}

// GetResponse returns the response by request ID
func GetResponse(requestID string) *sync.Map {
	response, ok := Response.Load(requestID)
	if !ok {
		return nil
	}
	responseAsMap, ok := response.(sync.Map)
	if !ok {
		return nil
	}
	return &responseAsMap
}

// InitResponse initializes the map to store response
func InitResponse(requestID string) {
	Response.Store(requestID, sync.Map{})
}

// AddKeyToResponse adds/updates a key in the response for a particular request id
func AddKeyToResponse(requestID string, key string, value interface{}) bool {
	responseMap := GetResponse(requestID)
	if responseMap != nil {
		responseMap.Store(key, value)
		return true
	}
	return false
}

// RemoveKeyToResponse removes a key in the response for a particular request id
func RemoveKeyToResponse(requestID string, key string) bool {
	responseMap := GetResponse(requestID)
	if responseMap != nil {
		responseMap.Delete(key)
		return true
	}
	return false
}

// ClearResponse clears the cache for a particular request ID
func ClearResponse(requestID string) {
	Response.Delete(requestID)
}
