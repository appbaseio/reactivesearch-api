package pipelines

import (
	"context"
	"sync"

	"github.com/appbaseio/reactivesearch-api/errors"
)

// contextKey is a key against which the pipeline log body
// will be stored in the context
type contextKey string

// ctxKey is the key of the value being stored in the context.
const ctxKey = contextKey("pipeline-log")

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, pipelineLog *PipelineLog) context.Context {
	return context.WithValue(ctx, ctxKey, pipelineLog)
}

// FromContext retrieves the pipeline log stored against the ctxKey from the context.
func FromContext(ctx context.Context) (*PipelineLog, error) {
	ctxRequest := ctx.Value(ctxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("PipelineLog")
	}
	reqBody, ok := ctxRequest.(*PipelineLog)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "PipelineLog")
	}
	return reqBody, nil
}

// CtxKey is a key against which wg will get stored in the context.
const WgCtxKey = contextKey("diff-update-wg")

// NewContext returns a context with the passed value stored against the
// context key.
func WgNewContext(ctx context.Context, wg *[]*sync.WaitGroup) context.Context {
	return context.WithValue(ctx, WgCtxKey, wg)
}

// FromContext retrieves waitgroup saved in the context.
func WgFromContext(ctx context.Context) (*[]*sync.WaitGroup, error) {
	ctxRequest := ctx.Value(WgCtxKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Context Diff Waitgroup")
	}
	changes, ok := ctxRequest.(*[]*sync.WaitGroup)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Context Diff Waitgroup")
	}
	return changes, nil
}

const PipelineValidateIndicatorKey = contextKey("pipeline-is-validate")

func PipelineIsValidateNewContext(ctx context.Context, isValidate *bool) context.Context {
	return context.WithValue(ctx, PipelineValidateIndicatorKey, isValidate)
}

func PipelineIsValidateFromContext(ctx context.Context) (*bool, error) {
	ctxRequest := ctx.Value(PipelineValidateIndicatorKey)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("Context Validate indicator key")
	}
	changes, ok := ctxRequest.(*bool)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "Context Validate indicator key")
	}
	return changes, nil
}
