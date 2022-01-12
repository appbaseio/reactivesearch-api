package responsechange

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/model/difference"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("response-changes")

// NewContext returns a context with the passed value stored against the
// context key.
func NewContext(ctx context.Context, request *[]difference.Difference) context.Context {
	return context.WithValue(ctx, CtxKey, request)
}

// FromContext retrieves the array of request changes saved in the context.
func FromContext(ctx context.Context) (*[]difference.Difference, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Response Changes")
	}
	changes, ok := ctxRequest.(*[]difference.Difference)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Response Changes")
	}
	return changes, nil
}
