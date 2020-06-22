package esresponse

// Response represents the cached API response for a request
// Key is the unique ID for each request
var Response = make(map[string]interface{})

// GetResponse returns the response by request ID
func GetResponse(requestID string) *interface{} {
	response, ok := Response[requestID]
	if !ok {
		return nil
	}
	return &response
}

// SaveResponse returns the response by request ID
func SaveResponse(requestID string, response interface{}) {
	Response[requestID] = response
}

// ClearResponse clears the cache for a particular request ID
func ClearResponse(requestID string) {
	delete(Response, requestID)
}
