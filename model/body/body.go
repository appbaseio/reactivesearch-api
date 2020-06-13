package body

import (
	"context"

	"github.com/appbaseio/arc/errors"
)

type contextKey string

// ctxKey is a key against which an body.Body is stored in the context.
const ctxKey = contextKey("body")

// NewContext returns a new context with the given ACL.
func NewContext(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, ctxKey, body)
}

// FromContext retrieves the acl stored against the acl.CtxKey from the context.
func FromContext(ctx context.Context) ([]byte, error) {
	ctxBody := ctx.Value(ctxKey)
	if ctxBody == nil {
		return nil, errors.NewNotFoundInContextError("body")
	}
	reqBody, ok := ctxBody.([]byte)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxBody", "string")
	}
	return reqBody, nil
}
