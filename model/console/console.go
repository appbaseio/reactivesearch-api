package console

import (
	"context"
	"fmt"
	"strings"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/appbaseio/reactivesearch-api/util"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("console-logs")

// NewContext returns a context with the passed value stored against the
// context key.
func NewContext(ctx context.Context, consoleStr *string) context.Context {
	// Parse the string and save it as an array
	consoleStrValue := *consoleStr
	consoleStrValue = string(consoleStrValue[:util.Min(len(consoleStrValue), 1000000)])
	consoleLogs := strings.Split(consoleStrValue, "\n")

	fmt.Println("passed: ", *consoleStr)
	fmt.Println("limited: ", consoleStrValue)
	fmt.Println("logs: ", consoleLogs)

	return context.WithValue(ctx, CtxKey, &consoleLogs)
}

// FromContext retrieves the logs saved in the context.
func FromContext(ctx context.Context) (*[]string, error) {
	ctxRequest := ctx.Value(CtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Console Logs")
	}
	changes, ok := ctxRequest.(*[]string)
	fmt.Println("changes: ", changes)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Console Logs")
	}
	return changes, nil
}
