package rules

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

// contextKey is a key against which rs api request will get stored in the context.
type contextKey string

// ctxKey is the key of the value being stored in the context.
const ctxKey = contextKey("rule-request")

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, request ScriptRequest) context.Context {
	return context.WithValue(ctx, ctxKey, request)
}

// FromContext retrieves the Script context request stored against the rules.ctxKey from the context.
func FromContext(ctx context.Context) (*ScriptRequest, error) {
	ctxRequest := ctx.Value(ctxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("ScriptRequest")
	}
	reqBody, ok := ctxRequest.(ScriptRequest)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "ScriptRequest")
	}
	return &reqBody, nil
}
