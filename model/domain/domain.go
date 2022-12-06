package domain

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
)

type contextKey string

type DomainInfo struct {
	Encrypted string
	Raw       string
}

// CtxKey is a key against which request domain will get stored in the context.
const CtxKey = contextKey("request-domain")

// NewContext returns a new context with the given domain name.
func NewContext(ctx context.Context, domainInfo DomainInfo) context.Context {
	return context.WithValue(ctx, CtxKey, domainInfo)
}

// FromContext retrieves encrypted domain value from the context.
func FromContext(ctx context.Context) (*DomainInfo, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Domain")
	}
	domainName, ok := ctxRequest.(DomainInfo)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Domain")
	}
	return &domainName, nil
}
