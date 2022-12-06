package domain

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

type contextKey string

// CtxKey is a key against which request domain will get stored in the context.
const CtxKey = contextKey("request-domain")

// NewContext returns a new context with the given domain name.
func NewContext(ctx context.Context, request string) context.Context {
	return context.WithValue(ctx, CtxKey, request)
}

// FromContext retrieves encrypted domain value from the context.
func FromContext(ctx context.Context) (*string, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Domain")
	}
	domainName, ok := ctxRequest.(string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Domain")
	}
	return &domainName, nil
}
