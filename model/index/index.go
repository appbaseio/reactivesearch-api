package index

import (
	"context"

	"github.com/appbaseio-confidential/arc/errors"
)

type contextKey string

// CtxKey is a key against which a slice of indices or index patterns are stored in the context.
const CtxKey = contextKey("indices")

// FromContext retrieves the slice of indices or index patterns stored against the index.CtxKey from the context.
func FromContext(ctx context.Context) ([]string, error) {
	ctxIndices := ctx.Value(CtxKey)
	if ctxIndices == nil {
		return nil, errors.NewNotFoundInContextError("indices")
	}
	reqIndices, ok := ctxIndices.([]string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxIndices", "[]string")
	}
	return reqIndices, nil
}

func NewContext(ctx context.Context, indices []string) context.Context {
	return context.WithValue(ctx, CtxKey, indices)
}
