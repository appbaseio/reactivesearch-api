package console

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("console-logs")

// NewContext returns a context with the passed value stored against the
// context key.
func NewContext(ctx context.Context, request *string) context.Context {
	return context.WithValue(ctx, CtxKey, request)
}

// FromContext retrieves the logs saved in the context.
func FromContext(ctx context.Context) (*string, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Console Logs")
	}
	changes, ok := ctxRequest.(*string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Console Logs")
	}
	return changes, nil
}
