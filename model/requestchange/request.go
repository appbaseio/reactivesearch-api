package requestchange

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/plugins/logs"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("request-changes")

// NewContext returns a context with the passed value stored against the
// context key.
func NewContext(ctx context.Context, request []logs.Difference) context.Context {
	return context.WithValue(ctx, CtxKey, request)
}

// FromContext retrieves the array of request changes saved in the context.
func FromContext(ctx context.Context) (*[]logs.Difference, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Request Changes")
	}
	changes, ok := ctxRequest.([]logs.Difference)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Request Changes")
	}
	return &changes, nil
}
