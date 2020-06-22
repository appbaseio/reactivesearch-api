package requestid

import (
	"context"

	"github.com/appbaseio/arc/errors"
	"github.com/google/uuid"
)

type contextKey string

// ctxKey is a key against which request id will get stored in the context.
const ctxKey = contextKey("requestid")

// NewContext returns a new context with the given request id.
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey, uuid.New().String())
}

// FromContext retrieves the api requestID stored against the requestID.ctxKey from the context.
func FromContext(ctx context.Context) (*string, error) {
	ctxRequestID := ctx.Value(ctxKey)
	if ctxRequestID == nil {
		return nil, errors.NewNotFoundInContextError("requestid")
	}
	requestID, ok := ctxRequestID.(string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequestID", "requestid")
	}
	return &requestID, nil
}
