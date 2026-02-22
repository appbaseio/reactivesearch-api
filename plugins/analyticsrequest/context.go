package analyticsrequest

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

type contextKey string

// ctxKey is a key against which analyticsrequest will get stored in the context.
const ctxKey = contextKey("analyticsrequest")

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, analyticsrequest Record) context.Context {
	return context.WithValue(ctx, ctxKey, analyticsrequest)
}

// FromContext retrieves the analyticsrequest against the ctxKey from the context.
func FromContext(ctx context.Context) (*Record, error) {
	ctxRequest := ctx.Value(ctxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("analyticsrequest")
	}
	analyticsrequest, ok := ctxRequest.(Record)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "analyticsrequest")
	}
	return &analyticsrequest, nil
}
