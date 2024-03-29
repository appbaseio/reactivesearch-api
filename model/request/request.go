package request

import (
	"context"

	"github.com/appbaseio/reactivesearch-api/errors"
	"github.com/google/uuid"
)

type contextKey string

// CtxKey is a key against which api request will get stored in the context.
const CtxKey = contextKey("original-request")

// NewContext returns a new context with the given request body.
func NewContext(ctx context.Context, request interface{}) context.Context {
	return context.WithValue(ctx, CtxKey, request)
}

// FromContext retrieves the api request body stored against the request.ctxKey from the context.
func FromContext(ctx context.Context) (*interface{}, error) {
	ctxRequest := ctx.Value(CtxKey)
	return &ctxRequest, nil
}

// ctxKeyRequestId is a key against which request ID would be stored in context.
const ctxKeyRequestId = contextKey("request-id")

type RequestInfo struct {
	Id   string
	Body []byte
}

// NewRequestIDContext returns a new context with request ID.
func NewRequestIDContext(ctx context.Context, originalRequestBody []byte) context.Context {
	requestId := uuid.New().String()
	requestInfo := RequestInfo{
		Id:   requestId,
		Body: originalRequestBody,
	}
	return context.WithValue(ctx, ctxKeyRequestId, requestInfo)
}

// FromContext retrieves the rs ap request Id stored against the querytranslate.ctxKeyRequestId from the context.
func FromRequestIDContext(ctx context.Context) (*RequestInfo, error) {
	ctxRequest := ctx.Value(ctxKeyRequestId)
	if ctxRequest == nil {
		return nil, errors.NewNotFoundInContextError("RequestId")
	}
	reqQuery, ok := ctxRequest.(RequestInfo)
	if !ok {
		return nil, errors.NewInvalidCastError("ctxRequest", "RequestId")
	}
	return &reqQuery, nil
}
