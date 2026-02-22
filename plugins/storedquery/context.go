package storedquery

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

type contextKey string

// ctxKey is a key against which storedqueries will get stored in the context.
const ctxKey = contextKey("storedqueries")

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, storedqueries []string) context.Context {
	return context.WithValue(ctx, ctxKey, storedqueries)
}

// FromContext retrieves the storedqueries against the ctxKey from the context.
func FromContext(ctx context.Context) ([]string, error) {
	ctxRequest := ctx.Value(ctxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("storedqueries")
	}
	storedqueries, ok := ctxRequest.([]string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "storedqueries")
	}
	return storedqueries, nil
}
