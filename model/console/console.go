package console

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/util"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("console-logs")

// NewContext returns a context with the passed value stored against the
// context key.
func NewContext(ctx context.Context, consoleStr *[]string) context.Context {
	return context.WithValue(ctx, CtxKey, consoleStr)
}

// FromContext retrieves the logs saved in the context.
func FromContext(ctx context.Context) (*[]string, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Console Logs")
	}
	consoleLogs, ok := ctxRequest.(*[]string)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Console Logs")
	}
	return consoleLogs, nil
}

// LimitConsoleString truncates the data if the string is more than
// 10KB and returns a new string
func LimitConsoleString(console string) string {
	return string(console[:util.Min(len(console), 1000000)])
}
