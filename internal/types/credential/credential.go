package credential

import (
	"context"

	"github.com/appbaseio-confidential/arc/internal/errors"
)

type contextKey string

// CtxKey is a key against which a request credential identifier is stored.
const CtxKey = contextKey("request_maker")

// Credential is a value stored in the context that identifies
// whether the request uses a user credential or permission credential.
type Credential int

// Credentials
const (
	User Credential = iota
	Permission
)

// FromContext retrieves credential type stored in the context against credential.CtxKey.
func FromContext(ctx context.Context) (Credential, error) {
	ctxCredential := ctx.Value(CtxKey)
	if ctxCredential == nil {
		return -1, errors.NewNotFoundInContextError("request.Credential")
	}
	reqCredential, ok := ctxCredential.(Credential)
	if !ok {
		return -1, errors.NewInvalidCastError("ctxCredential", "request.Credential")
	}
	return reqCredential, nil
}
