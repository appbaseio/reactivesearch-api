package response

import (
	"context"

	"github.com/appbaseio/arc/errors"
)

// // Response represents the cached API response for a request
// // Key is the unique ID for each request
// var Response = make(map[string]map[string]interface{})

// // GetResponse returns the response by request ID
// func GetResponse(requestID string) *map[string]interface{} {
// 	response, ok := Response[requestID]
// 	if !ok {
// 		return nil
// 	}
// 	return &response
// }

// // SaveResponse returns the response by request ID
// func SaveResponse(requestID string, response map[string]interface{}) {
// 	Response[requestID] = response
// }

// // ClearResponse clears the cache for a particular request ID
// func ClearResponse(requestID string) {
// 	delete(Response, requestID)
// }

type contextKey string

// CtxKey is a key against which api response will get stored in the context.
const ctxKey = contextKey("response")

// NewContext returns a new context with the given response body.
func NewContext(ctx context.Context, response map[string]interface{}) context.Context {
	return context.WithValue(ctx, ctxKey, response)
}

// FromContext retrieves the api response body stored against the response.ctxKey from the context.
func FromContext(ctx context.Context) (*map[string]interface{}, error) {
	ctxResponse := ctx.Value(ctxKey)
	if ctxResponse == nil {
		return nil, errors.NewNotFoundInContextError("response")
	}
	response, ok := ctxResponse.(map[string]interface{})
	if !ok {
		return nil, errors.NewInvalidCastError("ctxResponse", "response")
	}
	return &response, nil
}
