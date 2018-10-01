package logger

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/appbaseio-confidential/arc/arc/middleware"
	"github.com/appbaseio-confidential/arc/arc/middleware/order"
)

const logTag = "[logger]"

type Logger struct {
	order.Single
}

func (l *Logger) Wrap(h http.HandlerFunc) http.HandlerFunc {
	return l.Adapt(h, New())
}

func New() middleware.Middleware {
	return logger
}

func logger(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		log.Println(fmt.Sprintf("%s: started %s", logTag, msg))
		start := time.Now()
		h(w, r)
		log.Println(fmt.Sprintf("%s: finished %s, took %fs", logTag, msg, time.Since(start).Seconds()))
	}
}
