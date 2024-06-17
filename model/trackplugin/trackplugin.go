package trackplugin

import (
	"context"
)

type contextKey string

// ctxKey is a key against which request stores a list of applied plugins.
const loggerCtxKey = contextKey("middleware_tracker")

// TrackPlugin adds a plugin to the list of applied plugins
func TrackPlugin(ctx context.Context, mw string) context.Context {
	appliedPlugins := FrompluginTrackerContext(ctx)
	return context.WithValue(ctx, loggerCtxKey, append(appliedPlugins, mw))
}

// FrompluginTrackerContext retrieves the applied plugins for a request
func FrompluginTrackerContext(ctx context.Context) []string {
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
