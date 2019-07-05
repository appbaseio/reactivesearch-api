package index

import (
	"context"

	"github.com/appbaseio/arc/errors"
)

type contextKey string

// ctxKey is a key against which a slice of indices or index patterns are stored in the context.
const ctxKey = contextKey("indices")

// NewContext returns a new context with the given indices.
func NewContext(ctx context.Context, indices []string) context.Context {
	return context.WithValue(ctx, ctxKey, indices)
}

// FromContext retrieves the slice of indices or index patterns stored against the index.CtxKey from the context.
func FromContext(ctx context.Context) ([]string, error) {
	ctxIndices := ctx.Value(ctxKey)
	if ctxIndices == nil {
		return nil, errors.NewNotFoundInContextError("indices")
	}
	reqIndices, ok := ctxIndices.([]string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxIndices", "[]string")
	}
	return reqIndices, nil
}
