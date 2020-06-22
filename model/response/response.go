package response

import (
	"context"

	"github.com/appbaseio/arc/errors"
)

type contextKey string

// ctxKey is a key against which api response will get stored in the context.
const ctxKey = contextKey("response")

// NewContext returns a new context with the given api response body.
func NewContext(ctx context.Context, response map[string]interface{}) context.Context {
	return context.WithValue(ctx, ctxKey, response)
}

// FromContext retrieves the api response stored against the response.ctxKey from the context.
func FromContext(ctx context.Context) (*map[string]interface{}, error) {
	ctxRequest := ctx.Value(ctxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("map[string]interface{}")
	}
	reqQuery, ok := ctxRequest.(map[string]interface{})
	if !ok {
		return nil, errors.NewInvalidCastError("ctxResponse", "map[string]interface{}")
	}
	return &reqQuery, nil
}
