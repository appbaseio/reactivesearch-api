package querytranslate

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

// ctxKey is a key against which rs api request will get stored in the context.
type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const ctxKey = contextKey("request")

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, rsQuery RSQuery) context.Context {
	return context.WithValue(ctx, ctxKey, rsQuery)
}

// FromContext retrieves the rs ap request stored against the querytranslate.ctxKey from the context.
func FromContext(ctx context.Context) (*RSQuery, error) {
	ctxRequest := ctx.Value(ctxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("RSQuery")
	}
	reqQuery, ok := ctxRequest.(RSQuery)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "RSQuery")
	}
	return &reqQuery, nil
}
