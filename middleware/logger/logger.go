package logger

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const logTag = "[logger]"

var (
	instance *logger
	once     sync.Once
)

type logger struct{}

func Instance() *logger {
	once.Do(func() { instance = &logger{} })
	return instance
}

func (l *logger) Log(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		msg := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		start := time.Now()
		h(w, r)
		log.Println(fmt.Sprintf("%s: finished %s, took %fs",
			logTag, msg, time.Since(start).Seconds()))
	}
}
