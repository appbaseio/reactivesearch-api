package queryid

import (
	"context"

	"github.com/appbaseio/arc/errors"
)

type contextKey string

// ctxKey is a key against which a slice of queryids are stored in the context.
const ctxKey = contextKey("queryid")

// NewContext returns a new context with the given queryids.
func NewContext(ctx context.Context, queryids []string) context.Context {
	return context.WithValue(ctx, ctxKey, queryids)
}

// FromContext retrieves the slice of queryids stored against the queryid.CtxKey from the context.
func FromContext(ctx context.Context) ([]string, error) {
	ctxQueryIds := ctx.Value(ctxKey)
	if ctxQueryIds == nil {
		return nil, errors.NewNotFoundInContextError("queryid")
	}
	reqQueryIds, ok := ctxQueryIds.([]string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxQueryIds", "[]string")
	}
	return reqQueryIds, nil
}
