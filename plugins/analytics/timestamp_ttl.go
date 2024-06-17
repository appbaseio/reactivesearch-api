package analytics

import (
	"sync"
	"time"
)

// userid to timestamp map
type timestampItem struct {
	value      int64
	lastAccess int64
}
type TTLMapTimestamp struct {
	m map[string]*timestampItem
	l sync.Mutex
}

func InitTimestampSession(ln int, maxTTL int) (m *TTLMapTimestamp) {
	m = &TTLMapTimestamp{m: make(map[string]*timestampItem, ln)}
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
	return
}

func (m *TTLMapTimestamp) Len() int {
	return len(m.m)
}

func (m *TTLMapTimestamp) Put(k string, v int64) {
	m.l.Lock()
	it := &timestampItem{value: v}
	m.m[k] = it
	it.lastAccess = time.Now().Unix()
	m.l.Unlock()
}

func (m *TTLMapTimestamp) Get(k string) (v int64) {
	m.l.Lock()
	if it, ok := m.m[k]; ok {
		v = it.value
		it.lastAccess = time.Now().Unix()
	}
	m.l.Unlock()
	return
}
