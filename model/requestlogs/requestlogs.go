package requestlogs

import (
	"log"
	"net/http"
	"sync"
	"time"
)

type RequestData struct {
	Body    string
	Headers http.Header
	URL     string
	Method  string
}

type LogsResults struct {
	LogType   string // can be `request` or `response`
	LogTime   string // can be `before` or `after`
	Data      RequestData
	Stage     string
	TimeTaken float64
}

type ActiveRequestLog struct {
	LogsDiffing *sync.WaitGroup
	Output      chan LogsResults
}

type ActiveRequestLogData struct {
	value      ActiveRequestLog
	lastAccess int64
}
type ActiveRequestLogsTTLMap struct {
	m map[string]*ActiveRequestLogData
	l sync.Mutex
}

// To store the request logs
var m *ActiveRequestLogsTTLMap

// Initialize request logs with maximum no. of requests & maxTTL in seconds
func InitRequestLogs(ln int, maxTTL int) {
	log.Println("Initializing request logs map")
	m = &ActiveRequestLogsTTLMap{m: make(map[string]*ActiveRequestLogData, ln)}
	go func() {
		for now := range time.Tick(time.Second) {
			m.l.Lock()
			for k, v := range m.m {
				if now.Unix()-v.lastAccess > int64(maxTTL) {
					delete(m.m, k)
				}
			}
			m.l.Unlock()
		}
	}()
}

// Init request logs by request Id
func Put(k string, v ActiveRequestLog) {
	m.l.Lock()
	it, ok := m.m[k]
	if !ok {
		it = &ActiveRequestLogData{value: v}
		m.m[k] = it
	}
	it.lastAccess = time.Now().Unix()
	m.l.Unlock()
}

// Get request logs by request Id
func Get(k string) (v *ActiveRequestLog) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = &it.value
		it.lastAccess = time.Now().Unix()
	}
	m.l.Unlock()
	return
}

// Delete request logs by request Id
func Delete(k string) (v *ActiveRequestLog) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = &it.value
		delete(m.m, k)
	}
	m.l.Unlock()
	return
}
