package logger

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/arc/arc"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
)

const Tag = "[logger]"

type Logger struct {
	order.Single
}

func (l *Logger) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return l.Adapt(h, New())
}

func New() arc.Middleware {
	return logger
}

func logger(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		log.Println(fmt.Sprintf("%s: started %s", Tag, msg))
		start := time.Now()
		defer log.Println(fmt.Sprintf("%s: finished %s, took %f", Tag, msg, time.Since(start).Seconds()))
		h(w, r)
	}
}
