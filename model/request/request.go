package request

import (
	"context"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("original-unmodified-request")

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, request interface{}) context.Context {
	println("Creating new context with original-request body: ", request)
	return context.WithValue(ctx, CtxKey, request)
}

// FromContext retrieves the api request body stored against the request.ctxKey from the context.
func FromContext(ctx context.Context) (*interface{}, error) {
	ctxRequest := ctx.Value(CtxKey)
	println("Getting original request from context: ", ctxRequest)
	return &ctxRequest, nil
}
