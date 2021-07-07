package querytranslate

import (
	"context"

	"github.com/appbaseio/arc/errors"
	"github.com/appbaseio/arc/model/request"
)

// ctxKey is a key against which rs api request will get stored in the context.
// Note: key is similar to arc-oss so logs can read it
const ctxKey = request.CtxKey

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
