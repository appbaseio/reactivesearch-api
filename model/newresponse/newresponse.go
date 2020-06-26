package newresponse

import (
	"context"
	"sync"

	"github.com/appbaseio/arc/errors"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("response")

type Response struct {
	L        *sync.RWMutex
	Response *sync.Map
}

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, response Response) context.Context {
	return context.WithValue(ctx, CtxKey, response)
}

// FromContext retrieves the api request body stored against the request.ctxKey from the context.
func FromContext(ctx context.Context) (*Response, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("*category.Categories")
	}
	reqACL, ok := ctxRequest.(Response)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxACL", "*category.Categories")
	}
	return reqACL, nil
}
