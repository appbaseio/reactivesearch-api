package trackmiddleware

import (
	"context"
)

type contextKey string

// ctxKey is a key against which request stores a list of applied middlewares.
const loggerCtxKey = contextKey("middleware_tracker")

// TrackMiddleware adds a middleware to the list of applied middlewares
func TrackMiddleware(ctx context.Context, mw string) context.Context {
	appliedMiddlewares := FromTimeTrackerContext(ctx)
	return context.WithValue(ctx, loggerCtxKey, append(appliedMiddlewares, mw))
}

// FromTimeTrackerContext retrieves the applied middlewares for a request
func FromTimeTrackerContext(ctx context.Context) []string {
	ctxRequest := ctx.Value(loggerCtxKey)
	if ctxRequest == nil {
		return []string{}
	}
	appliedMws, ok := ctxRequest.([]string)
	if !ok {
		return []string{}
	}
	return appliedMws
}
