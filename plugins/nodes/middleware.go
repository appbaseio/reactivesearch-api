package nodes

import (
	"net/http"

	"github.com/appbaseio/reactivesearch-api/middleware"
)

type chain struct {
	middleware.Fifo
}

func (c *chain) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return c.Adapt(h, list()...)
}

func list() []middleware.Middleware {
	return []middleware.Middleware{}
}
