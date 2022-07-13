package querytranslate

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

// ctxKey is a key against which rs api request will get stored in the context.
type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const ctxKey = contextKey("request")

const independentReqCtxKey = contextKey("independent-request")

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

// NewIndependentRequestContext returns a new context with the
// given independent request body.
func NewIndependentRequestContext(ctx context.Context, independentRequests []map[string]interface{}) context.Context {
	return context.WithValue(ctx, independentReqCtxKey, independentRequests)
}

// FromIndependentRequestContext retrieves the rs api request stored
// against the querytranslate.ctxKey from the context.
func FromIndependentRequestContext(ctx context.Context) (*[]map[string]interface{}, error) {
	ctxRequest := ctx.Value(independentReqCtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Independent RSQuery")
	}
	reqQuery, ok := ctxRequest.([]map[string]interface{})
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Independent RSQuery")
	}
	return &reqQuery, nil
}
