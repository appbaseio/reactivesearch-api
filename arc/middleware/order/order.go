package order

import (
	"net/http"

	"github.com/appbaseio-confidential/arc/arc/middleware"
)

// Fifo is a type that implements Adapter. It provides
// a First-In, First-Out ordering of the middleware.
type Fifo string

// Adapt adapts the handler in First-In, First-Out manner.
// The request will pass through the middleware in the sequence
// in which they are passed in the function.
func (f *Fifo) Adapt(h http.HandlerFunc, m ...middleware.Middleware) http.HandlerFunc {
	for i := len(m) - 1; i >= 0; i-- {
		h = m[i](h)
	}
	return h
}

// Lifo is a type that implements Adapter. It provides
// a Last-In, First-Out ordering of the middleware.
type Lifo string

// Adapt adapts the handler in Last-In, First-Out manner.
// The request will pass through the middleware in the opposite
// sequence in which they are passed.
func (l *Lifo) Adapt(h http.HandlerFunc, m ...middleware.Middleware) http.HandlerFunc {
	for i := 0; i < len(m); i++ {
		h = m[i](h)
	}
	return h
}

// Single is a type that implements Adapter. It provides
// a simplest of ordering in which a handler is adapted to
// a single middleware. Embedding Single type makes it clear
// that a chain of middleware deals with only a single middleware.
type Single string

// Adapt adapts the handler to a single middleware.
func (s *Single) Adapt(h http.HandlerFunc, m ...middleware.Middleware) http.HandlerFunc {
	if len(m) != 0 {
		return m[0](h)
	}
	return h
}
