package index

import (
	"context"

	"github.com/appbaseio-confidential/arc/internal/errors"
)

type contextKey string

const CtxKey = contextKey("indices")

func FromContext(ctx context.Context) ([]string, error) {
	ctxIndices := ctx.Value(CtxKey)
	if ctxIndices == nil {
		return nil, errors.NewNotFoundInRequestContextError("indices")
	}
	reqIndices, ok := ctxIndices.([]string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxIndices", "[]string")
	}
	return reqIndices, nil
}
