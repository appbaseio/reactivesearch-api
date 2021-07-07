package querytranslate

import (
	"context"
	"time"

	"github.com/appbaseio/arc/errors"
)

type contextKey string

// ctxKey is a key against which rs api request store the start time for the request.
const loggerCtxKey = contextKey("time_tracker")

// NewContext returns a new context with the given request time.
func NewLoggerContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, loggerCtxKey, time.Now())
}

// FromContext retrieves the rs api request stored against the querytranslate.loggerCtxKey from the context.
func FromLoggerContext(ctx context.Context) (*time.Time, error) {
	ctxRequest := ctx.Value(loggerCtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("RSQueryTimeTracker")
	}
	reqQuery, ok := ctxRequest.(time.Time)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequestLogger", "RSQueryTimeTracker")
	}
	return &reqQuery, nil
}
