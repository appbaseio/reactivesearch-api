package middleware

import "net/http"

// Middleware is a type that represents a middleware function. A
// middleware usually operates on the request before and after the
// request is served.
type Middleware func(http.HandlerFunc) http.HandlerFunc

// Chainer is implemented by any value that has a Wrap method,
// which wraps a single middleware or a 'chain' of middleware on
// top of the given handler function and returns the resulting
// 'wrapped' handler function.
type Chainer interface {
	// Wrap wraps a handler in a single middleware or a set of middleware.
	// The sequence of in which the handler is wrapped can be defined
	// manually or it can be defined by an Adapter.
	Wrap(http.HandlerFunc) http.HandlerFunc
}

// Adapter is implemented by any value that has Adapt method.
// Adapter is a type that can 'adapt' a given handler to the
// set of middleware in a specific order.
type Adapter interface {
	// Adapt adapts a handler to a given set of middleware in a
	// specific order. The request gets passed on to the next
	// middleware according to the order in which the handler is
	// adapted.
	Adapt(http.HandlerFunc, ...Middleware) http.HandlerFunc
}
