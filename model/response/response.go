package response

import (
	"context"
	"sync"

	"github.com/appbaseio/arc/errors"
)

type contextKey string

// CtxKey is a key against which api response will get stored in the context.
const ctxKey = contextKey("response")

type Response struct {
	L        *sync.RWMutex
	Command  chan string // use to control the go routine executions
	Response []byte
}

// NewContext returns a new context with the given response body.
func NewContext(ctx context.Context, response Response) context.Context {
	return context.WithValue(ctx, ctxKey, response)
}

// FromContext retrieves the api response body stored against the response.ctxKey from the context.
func FromContext(ctx context.Context) (*Response, error) {
	ctxResponse := ctx.Value(ctxKey)
	if ctxResponse == nil {
		return nil, errors.NewNotFoundInContextError("Response")
	}
	responseBody, ok := ctxResponse.(Response)
	if !ok {
		return nil, errors.NewInvalidCastError("responseBody", "Response")
	}
	return &responseBody, nil
}
