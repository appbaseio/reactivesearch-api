package difference

import (
	"context"

	"github.com/appbaseio-confidential/reactivesearch/errors"
)

type Difference struct {
	URI     string   `json:"uri"`
	Headers string   `json:"headers"`
	Body    string   `json:"body"`
	Method  string   `json:"method"`
	Stage   string   `json:"stage"`
	Took    *float64 `json:"took,omitempty"`
}

type contextKey string

// CtxKey is a key against which disableLogging will get stored in the context.
const CtxKey = contextKey("disable-logs")

// NewContext returns a context with the passed value stored against the
// context key.
func NewContext(ctx context.Context, consoleStr *bool) context.Context {
	return context.WithValue(ctx, CtxKey, consoleStr)
}

// FromContext retrieves the disable-logs value saved in the context.
func FromContext(ctx context.Context) (*bool, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Disable Logs")
	}
	consoleLogs, ok := ctxRequest.(*bool)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Disable Logs")
	}
	return consoleLogs, nil
}
