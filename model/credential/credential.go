package credential

import (
	"context"

	"github.com/appbaseio-confidential/arc/errors"
)

type contextKey string

// ctxKey is a key against which a request credential identifier is stored.
const ctxKey = contextKey("request_credential")

// Credential is a value stored in the context that identifies
// whether the request uses a user credential or permission credential.
type Credential int

// Credentials
const (
	User Credential = iota
	Permission
)

// NewContext returns a new context carrying credential 'c'.
func NewContext(ctx context.Context, c Credential) context.Context {
	return context.WithValue(ctx, ctxKey, c)
}

// FromContext retrieves credential type stored in the context against credential.CtxKey.
func FromContext(ctx context.Context) (Credential, error) {
	ctxCredential := ctx.Value(ctxKey)
	if ctxCredential == nil {
		return -1, errors.NewNotFoundInContextError("request.Credential")
	}
	reqCredential, ok := ctxCredential.(Credential)
	if !ok {
		return -1, errors.NewInvalidCastError("ctxCredential", "request.Credential")
	}
	return reqCredential, nil
}
